use crate::modules::tasks::domain::models::agent_event::DialogAnswer;

/// Modo de entrega de un mensaje del usuario al agente.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MessageMode {
    /// Si el agente está ocupado, se entrega tras las tool calls en curso.
    Steer,
    /// Si el agente está ocupado, se entrega cuando termine.
    FollowUp,
}

/// Órdenes que se envían a un agente en ejecución.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AgentCommand {
    Prompt {
        message: String,
    },
    Steer {
        message: String,
    },
    FollowUp {
        message: String,
    },
    Abort,
    AnswerDialog {
        dialog_id: String,
        answer: DialogAnswer,
    },
}
