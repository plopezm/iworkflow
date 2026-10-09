//! Screen state of the printable diff report (plain or with the agent's explanations).

use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::MessageModeDto;
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::SendTaskMessage;
use crate::modules::tasks::presentation::state::diff_report::{
    explanation_prompt, find_explanation,
};
use crate::modules::tasks::presentation::state::transcript::Transcript;

/// Time given to the browser to paint the loading dialog before rendering the report.
#[cfg(feature = "hydrate")]
const PREPARE_DELAY: std::time::Duration = std::time::Duration::from_millis(80);

/// What the user asked for.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ReportRequest {
    None,
    Plain,
    Explained,
}

/// What the screen must show for the current request.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReportPhase {
    Idle,
    /// Content is available but the report is not rendered yet (the progress dialog is shown).
    Preparing,
    Plain,
    Waiting,
    Ready(String),
    /// Asking the agent failed; carries the error message.
    Failed(String),
}

impl ReportPhase {
    /// True while the agent is still producing the explanation.
    pub fn is_waiting(&self) -> bool {
        matches!(self, ReportPhase::Waiting)
    }

    /// True while the report is waiting to be rendered.
    pub fn is_preparing(&self) -> bool {
        matches!(self, ReportPhase::Preparing)
    }

    /// True while the progress dialog must be shown (waiting for the agent or preparing).
    pub fn is_busy(&self) -> bool {
        self.is_waiting() || self.is_preparing()
    }

    /// True when the report content is complete and can be rendered.
    fn has_report(&self) -> bool {
        matches!(self, ReportPhase::Plain | ReportPhase::Ready(_))
    }
}

/// Holds a complete report back as [`ReportPhase::Preparing`] until `rendered` is set.
///
/// Both report kinds go through this step so the loading dialog is painted before the heavy
/// render blocks the browser.
pub fn gate_render(phase: ReportPhase, rendered: bool) -> ReportPhase {
    if phase.has_report() && !rendered {
        ReportPhase::Preparing
    } else {
        phase
    }
}

/// Maps a request, the agent's answer (if any) and the send error (if any) to the phase to render.
///
/// An answer found in the transcript wins over a send error.
pub fn report_phase(
    request: ReportRequest,
    explanation: Option<String>,
    send_error: Option<String>,
) -> ReportPhase {
    match (request, explanation, send_error) {
        (ReportRequest::None, _, _) => ReportPhase::Idle,
        (ReportRequest::Plain, _, _) => ReportPhase::Plain,
        (ReportRequest::Explained, Some(text), _) => ReportPhase::Ready(text),
        (ReportRequest::Explained, None, Some(message)) => ReportPhase::Failed(message),
        (ReportRequest::Explained, None, None) => ReportPhase::Waiting,
    }
}

/// Error of an action result that resolved after `baseline` (the version seen at dispatch time).
///
/// Results at or before the baseline belong to earlier dispatches of the shared action.
pub fn error_since<T>(
    version: usize,
    baseline: usize,
    value: Option<&Result<T, ServerFnError>>,
) -> Option<String> {
    if version <= baseline {
        return None;
    }
    value
        .and_then(|result: &Result<T, ServerFnError>| result.as_ref().err())
        .map(|e: &ServerFnError| e.to_string())
}

#[derive(Clone, Copy)]
pub struct DiffReportState {
    request: RwSignal<ReportRequest>,
    /// Transcript length when the explanation was requested; older answers are ignored.
    since: RwSignal<usize>,
    /// Set one tick after the content is complete; until then the dialog stays up.
    rendered: RwSignal<bool>,
    /// Bumped on every new request or close, so stale render timers are ignored.
    epoch: RwSignal<usize>,
    transcript: RwSignal<Transcript>,
    dispatch: Callback<String>,
    pub phase: Memo<ReportPhase>,
}

impl DiffReportState {
    /// Adapter over [`Self::from_parts`] that sends the prompt through the shared `send` action.
    pub fn new(
        task_id: StoredValue<String>,
        transcript: RwSignal<Transcript>,
        send: ServerAction<SendTaskMessage>,
    ) -> Self {
        // Version of `send` when the last explanation was requested.
        let baseline: RwSignal<usize> = RwSignal::new(0);
        let version: RwSignal<usize> = send.version();
        let send_error: Signal<Option<String>> = Signal::derive(move || {
            let current: usize = version.get();
            send.value()
                .with(|result| error_since(current, baseline.get(), result.as_ref()))
        });
        let dispatch: Callback<String> = Callback::new(move |message: String| {
            baseline.set(version.get_untracked());
            send.dispatch(SendTaskMessage {
                id: task_id.get_value(),
                message,
                images: Vec::new(),
                mode: MessageModeDto::FollowUp,
            });
        });
        Self::from_parts(transcript, send_error, dispatch)
    }

    /// Core state built from plain inputs.
    ///
    /// `send_error` must only report failures of sends issued through `dispatch`.
    pub fn from_parts(
        transcript: RwSignal<Transcript>,
        send_error: Signal<Option<String>>,
        dispatch: Callback<String>,
    ) -> Self {
        let request: RwSignal<ReportRequest> = RwSignal::new(ReportRequest::None);
        let since: RwSignal<usize> = RwSignal::new(0);
        let rendered: RwSignal<bool> = RwSignal::new(false);
        let epoch: RwSignal<usize> = RwSignal::new(0);
        let content: Memo<ReportPhase> = Memo::new(move |_| {
            let current: ReportRequest = request.get();
            if current != ReportRequest::Explained {
                return report_phase(current, None, None);
            }
            let prompt: String = explanation_prompt();
            let explanation: Option<String> =
                transcript.with(|t: &Transcript| find_explanation(t, &prompt, since.get()));
            report_phase(current, explanation, send_error.get())
        });
        let phase: Memo<ReportPhase> =
            Memo::new(move |_| gate_render(content.get(), rendered.get()));
        let state: Self = Self {
            request,
            since,
            rendered,
            epoch,
            transcript,
            dispatch,
            phase,
        };
        // Browser only: gives the dialog one tick to paint before the report is rendered.
        #[cfg(feature = "hydrate")]
        Effect::new(move |_| {
            let is_rendered: bool = rendered.get();
            if content.with(ReportPhase::has_report) && !is_rendered {
                state.schedule_render();
            }
        });
        state
    }

    #[cfg(feature = "hydrate")]
    fn schedule_render(&self) {
        let state: Self = *self;
        let scheduled_epoch: usize = self.epoch.get_untracked();
        set_timeout(
            move || state.render_if_current(scheduled_epoch),
            PREPARE_DELAY,
        );
    }

    #[cfg(any(feature = "hydrate", test))]
    /// Timer entry point: renders only if no request or close happened since `scheduled_epoch`.
    ///
    /// The timer can fire after the screen was torn down, so it never panics on disposed signals.
    fn render_if_current(&self, scheduled_epoch: usize) {
        if self.epoch.try_get_untracked() == Some(scheduled_epoch) {
            self.finish_preparing();
        }
    }

    /// Starts a new request: invalidates pending render timers and hides any rendered report.
    fn begin(&self, request: ReportRequest) {
        self.epoch.update(|e: &mut usize| *e += 1);
        self.rendered.set(false);
        self.request.set(request);
    }

    /// Opens the plain report (behind the loading dialog).
    pub fn open_plain(&self) {
        self.begin(ReportRequest::Plain);
    }

    /// Lets the complete report render, unless the user closed it in the meantime.
    pub fn finish_preparing(&self) {
        if self
            .request
            .try_get_untracked()
            .is_some_and(|request: ReportRequest| request != ReportRequest::None)
        {
            let _ = self.rendered.try_set(true);
        }
    }

    /// Asks the agent to explain the changes; the report opens once it answers.
    pub fn request_explanation(&self) {
        self.since.set(
            self.transcript
                .with_untracked(|t: &Transcript| t.items.len()),
        );
        self.begin(ReportRequest::Explained);
        self.dispatch.run(explanation_prompt());
    }

    /// Closes the report or stops waiting for the agent.
    pub fn close(&self) {
        self.begin(ReportRequest::None);
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::tasks::presentation::state::transcript::TranscriptItem;
    use any_spawner::Executor;
    use std::sync::{Arc, Mutex};

    fn user(text: &str) -> TranscriptItem {
        TranscriptItem::User {
            text: text.to_string(),
            images: 0,
        }
    }

    fn assistant(text: &str) -> TranscriptItem {
        TranscriptItem::Assistant {
            text: text.to_string(),
            thinking: None,
            error: None,
        }
    }

    /// Fixture: state wired to plain signals and a dispatch recorder.
    struct Fixture {
        state: DiffReportState,
        transcript: RwSignal<Transcript>,
        send_error: RwSignal<Option<String>>,
        sent: Arc<Mutex<Vec<String>>>,
    }

    fn fixture(initial: Vec<TranscriptItem>) -> Fixture {
        let transcript: RwSignal<Transcript> = RwSignal::new(Transcript {
            items: initial,
            ..Transcript::default()
        });
        let send_error: RwSignal<Option<String>> = RwSignal::new(None);
        let sent: Arc<Mutex<Vec<String>>> = Arc::new(Mutex::new(Vec::new()));
        let recorder: Arc<Mutex<Vec<String>>> = Arc::clone(&sent);
        // Like the real adapter, a new dispatch discards the results of earlier ones.
        let dispatch: Callback<String> = Callback::new(move |message: String| {
            if let Ok(mut messages) = recorder.lock() {
                messages.push(message);
            }
            send_error.set(None);
        });
        let state: DiffReportState =
            DiffReportState::from_parts(transcript, Signal::from(send_error), dispatch);
        Fixture {
            state,
            transcript,
            send_error,
            sent,
        }
    }

    fn push(transcript: RwSignal<Transcript>, item: TranscriptItem) {
        transcript.update(|t: &mut Transcript| t.items.push(item));
    }

    fn sent_messages(fx: &Fixture) -> Vec<String> {
        fx.sent.lock().map(|m| m.clone()).unwrap_or_default()
    }

    async fn with_owner(test: impl FnOnce()) {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(test);
    }

    #[test]
    fn idle_when_nothing_requested() {
        assert_eq!(
            report_phase(
                ReportRequest::None,
                Some("x".to_string()),
                Some("e".to_string())
            ),
            ReportPhase::Idle
        );
    }

    #[test]
    fn plain_ignores_explanation_and_error() {
        assert_eq!(
            report_phase(ReportRequest::Plain, None, Some("e".to_string())),
            ReportPhase::Plain
        );
    }

    #[test]
    fn explained_waits_until_the_agent_answers() {
        assert_eq!(
            report_phase(ReportRequest::Explained, None, None),
            ReportPhase::Waiting
        );
        assert_eq!(
            report_phase(ReportRequest::Explained, Some("texto".to_string()), None),
            ReportPhase::Ready("texto".to_string())
        );
    }

    #[test]
    fn explained_fails_on_send_error_but_an_answer_wins() {
        assert_eq!(
            report_phase(ReportRequest::Explained, None, Some("boom".to_string())),
            ReportPhase::Failed("boom".to_string())
        );
        assert_eq!(
            report_phase(
                ReportRequest::Explained,
                Some("texto".to_string()),
                Some("boom".to_string())
            ),
            ReportPhase::Ready("texto".to_string())
        );
    }

    #[test]
    fn only_waiting_phase_is_waiting() {
        assert!(ReportPhase::Waiting.is_waiting());
        assert!(!ReportPhase::Idle.is_waiting());
        assert!(!ReportPhase::Plain.is_waiting());
        assert!(!ReportPhase::Ready("x".to_string()).is_waiting());
        assert!(!ReportPhase::Failed("e".to_string()).is_waiting());
    }

    #[tokio::test]
    async fn stale_render_timer_is_ignored() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            let stale: usize = fx.state.epoch.get_untracked();
            // A newer request invalidates the timer scheduled for the previous one.
            fx.state.open_plain();
            fx.state.render_if_current(stale);
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Preparing);
            fx.state.render_if_current(fx.state.epoch.get_untracked());
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Plain);
        })
        .await;
    }

    #[tokio::test]
    async fn render_timer_after_close_does_nothing() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            let scheduled: usize = fx.state.epoch.get_untracked();
            fx.state.close();
            fx.state.render_if_current(scheduled);
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Idle);
        })
        .await;
    }

    #[tokio::test]
    async fn render_timer_after_the_screen_is_gone_does_not_panic() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        let state: DiffReportState = owner.with(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            fx.state
        });
        let scheduled: usize = owner.with(|| state.epoch.get_untracked());
        owner.cleanup();
        state.render_if_current(scheduled);
        state.finish_preparing();
    }

    #[test]
    fn complete_reports_wait_for_the_render_gate() {
        let ready: ReportPhase = ReportPhase::Ready("x".to_string());
        assert_eq!(
            gate_render(ReportPhase::Plain, false),
            ReportPhase::Preparing
        );
        assert_eq!(gate_render(ready.clone(), false), ReportPhase::Preparing);
        assert_eq!(gate_render(ReportPhase::Plain, true), ReportPhase::Plain);
        assert_eq!(gate_render(ready.clone(), true), ready);
    }

    #[test]
    fn gate_leaves_incomplete_phases_alone() {
        for phase in [
            ReportPhase::Idle,
            ReportPhase::Waiting,
            ReportPhase::Failed("e".to_string()),
        ] {
            assert_eq!(gate_render(phase.clone(), false), phase);
        }
    }

    #[test]
    fn busy_covers_waiting_and_preparing_only() {
        assert!(ReportPhase::Preparing.is_busy());
        assert!(ReportPhase::Waiting.is_busy());
        assert!(!ReportPhase::Plain.is_busy());
        assert!(!ReportPhase::Ready("x".to_string()).is_busy());
        assert!(!ReportPhase::Idle.is_busy());
    }

    #[tokio::test]
    async fn reopening_an_open_report_goes_through_the_dialog_again() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            fx.state.finish_preparing();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Plain);
            fx.state.open_plain();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Preparing);
        })
        .await;
    }

    #[tokio::test]
    async fn cancelling_while_preparing_prevents_the_render() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            fx.state.close();
            fx.state.finish_preparing();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Idle);
        })
        .await;
    }

    #[tokio::test]
    async fn cancelling_while_waiting_leaves_the_waiting_phase() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.request_explanation();
            assert!(fx.state.phase.get_untracked().is_waiting());
            fx.state.close();
            assert!(!fx.state.phase.get_untracked().is_waiting());
        })
        .await;
    }

    #[test]
    fn error_since_ignores_results_not_newer_than_the_baseline() {
        let failed: Result<(), ServerFnError> = Err(ServerFnError::new("boom"));
        let ok: Result<(), ServerFnError> = Ok(());
        assert_eq!(error_since(3, 3, Some(&failed)), None);
        assert_eq!(error_since(2, 3, Some(&failed)), None);
        assert_eq!(error_since(4, 3, Some(&ok)), None);
        assert_eq!(error_since(4, 3, None::<&Result<(), ServerFnError>>), None);
        let message: Option<String> = error_since(4, 3, Some(&failed));
        assert!(message.is_some_and(|m: String| m.contains("boom")));
    }

    #[tokio::test]
    async fn request_dispatches_the_prompt_once_and_waits() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Idle);
            fx.state.request_explanation();
            assert_eq!(sent_messages(&fx), vec![explanation_prompt()]);
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Waiting);
        })
        .await;
    }

    #[tokio::test]
    async fn matching_answer_moves_to_ready() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.request_explanation();
            push(fx.transcript, user(&explanation_prompt()));
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Waiting);
            push(fx.transcript, assistant("Explicación"));
            // Same pipeline as the plain report: the dialog stays up for one more tick.
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Preparing);
            fx.state.finish_preparing();
            assert_eq!(
                fx.state.phase.get_untracked(),
                ReportPhase::Ready("Explicación".to_string())
            );
        })
        .await;
    }

    #[tokio::test]
    async fn answer_from_before_the_request_is_ignored() {
        with_owner(|| {
            let fx: Fixture = fixture(vec![user(&explanation_prompt()), assistant("Vieja")]);
            fx.state.request_explanation();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Waiting);
        })
        .await;
    }

    #[tokio::test]
    async fn send_error_moves_to_failed() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.request_explanation();
            fx.send_error.set(Some("boom".to_string()));
            assert_eq!(
                fx.state.phase.get_untracked(),
                ReportPhase::Failed("boom".to_string())
            );
        })
        .await;
    }

    #[tokio::test]
    async fn error_older_than_the_request_is_ignored() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            // A failure of the shared action before the request is discarded by the dispatch.
            fx.send_error.set(Some("viejo".to_string()));
            fx.state.request_explanation();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Waiting);
        })
        .await;
    }

    #[tokio::test]
    async fn explanation_beats_error() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.request_explanation();
            push(fx.transcript, user(&explanation_prompt()));
            push(fx.transcript, assistant("Explicación"));
            fx.send_error.set(Some("boom".to_string()));
            fx.state.finish_preparing();
            assert_eq!(
                fx.state.phase.get_untracked(),
                ReportPhase::Ready("Explicación".to_string())
            );
        })
        .await;
    }

    #[tokio::test]
    async fn close_returns_to_idle_and_plain_opens_plain() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.open_plain();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Preparing);
            fx.state.finish_preparing();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Plain);
            fx.state.request_explanation();
            fx.state.close();
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Idle);
        })
        .await;
    }

    #[tokio::test]
    async fn retry_after_failure_redispatches_and_waits() {
        with_owner(|| {
            let fx: Fixture = fixture(Vec::new());
            fx.state.request_explanation();
            fx.send_error.set(Some("boom".to_string()));
            assert!(matches!(
                fx.state.phase.get_untracked(),
                ReportPhase::Failed(_)
            ));
            fx.state.request_explanation();
            assert_eq!(sent_messages(&fx).len(), 2);
            assert_eq!(fx.state.phase.get_untracked(), ReportPhase::Waiting);
        })
        .await;
    }
}
