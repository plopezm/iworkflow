//! Screen state of the printable diff report (plain or with the agent's explanations).

use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::MessageModeDto;
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::SendTaskMessage;
use crate::modules::tasks::presentation::state::diff_report::{
    explanation_prompt, find_explanation,
};
use crate::modules::tasks::presentation::state::transcript::Transcript;

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
    Plain,
    Waiting,
    Ready(String),
    /// Asking the agent failed; carries the error message.
    Failed(String),
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
        let phase: Memo<ReportPhase> = Memo::new(move |_| {
            let current: ReportRequest = request.get();
            if current != ReportRequest::Explained {
                return report_phase(current, None, None);
            }
            let prompt: String = explanation_prompt();
            let explanation: Option<String> =
                transcript.with(|t: &Transcript| find_explanation(t, &prompt, since.get()));
            report_phase(current, explanation, send_error.get())
        });
        Self {
            request,
            since,
            transcript,
            dispatch,
            phase,
        }
    }

    pub fn open_plain(&self) {
        self.request.set(ReportRequest::Plain);
    }

    /// Asks the agent to explain the changes; the report opens once it answers.
    pub fn request_explanation(&self) {
        self.since.set(
            self.transcript
                .with_untracked(|t: &Transcript| t.items.len()),
        );
        self.request.set(ReportRequest::Explained);
        self.dispatch.run(explanation_prompt());
    }

    /// Closes the report or stops waiting for the agent.
    pub fn close(&self) {
        self.request.set(ReportRequest::None);
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
