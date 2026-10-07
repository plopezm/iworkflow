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
}

/// Maps a request and the agent's answer (if any) to the phase to render.
pub fn report_phase(request: ReportRequest, explanation: Option<String>) -> ReportPhase {
    match (request, explanation) {
        (ReportRequest::None, _) => ReportPhase::Idle,
        (ReportRequest::Plain, _) => ReportPhase::Plain,
        (ReportRequest::Explained, None) => ReportPhase::Waiting,
        (ReportRequest::Explained, Some(text)) => ReportPhase::Ready(text),
    }
}

#[derive(Clone, Copy)]
pub struct DiffReportState {
    request: RwSignal<ReportRequest>,
    /// Transcript length when the explanation was requested; older answers are ignored.
    since: RwSignal<usize>,
    transcript: RwSignal<Transcript>,
    send: ServerAction<SendTaskMessage>,
    task_id: StoredValue<String>,
    pub phase: Memo<ReportPhase>,
}

impl DiffReportState {
    pub fn new(
        task_id: StoredValue<String>,
        transcript: RwSignal<Transcript>,
        send: ServerAction<SendTaskMessage>,
    ) -> Self {
        let request: RwSignal<ReportRequest> = RwSignal::new(ReportRequest::None);
        let since: RwSignal<usize> = RwSignal::new(0);
        let phase: Memo<ReportPhase> = Memo::new(move |_| {
            let current: ReportRequest = request.get();
            let explanation: Option<String> = if current == ReportRequest::Explained {
                let prompt: String = explanation_prompt();
                transcript.with(|t: &Transcript| find_explanation(t, &prompt, since.get()))
            } else {
                None
            };
            report_phase(current, explanation)
        });
        Self {
            request,
            since,
            transcript,
            send,
            task_id,
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
        self.send.dispatch(SendTaskMessage {
            id: self.task_id.get_value(),
            message: explanation_prompt(),
            mode: MessageModeDto::FollowUp,
        });
    }

    /// Closes the report or stops waiting for the agent.
    pub fn close(&self) {
        self.request.set(ReportRequest::None);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn idle_when_nothing_requested() {
        assert_eq!(
            report_phase(ReportRequest::None, Some("x".to_string())),
            ReportPhase::Idle
        );
    }

    #[test]
    fn plain_ignores_explanation() {
        assert_eq!(report_phase(ReportRequest::Plain, None), ReportPhase::Plain);
    }

    #[test]
    fn explained_waits_until_the_agent_answers() {
        assert_eq!(
            report_phase(ReportRequest::Explained, None),
            ReportPhase::Waiting
        );
        assert_eq!(
            report_phase(ReportRequest::Explained, Some("texto".to_string())),
            ReportPhase::Ready("texto".to_string())
        );
    }
}
