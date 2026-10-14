use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SubagentStatusDto {
    Queued,
    Running,
    Waiting,
    Completed,
    Failed,
    Cancelled,
    TimedOut,
    Unknown,
}

impl SubagentStatusDto {
    /// Sigue en cola, ejecutándose o esperando respuesta.
    pub fn is_active(&self) -> bool {
        matches!(self, Self::Queued | Self::Running | Self::Waiting)
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum SubagentThreadItemDto {
    Text {
        text: String,
    },
    Thinking {
        text: String,
    },
    Note {
        text: String,
    },
    Tool {
        name: String,
        args: Option<String>,
        running: bool,
        is_error: bool,
        output: Option<String>,
    },
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SubagentTaskDto {
    pub id: String,
    pub agent: String,
    pub label: Option<String>,
    pub prompt: Option<String>,
    pub status: SubagentStatusDto,
    pub created_at: Option<i64>,
    pub started_at: Option<i64>,
    pub ended_at: Option<i64>,
    pub last_step: Option<String>,
    pub last_activity_at: Option<i64>,
    pub turns: u32,
    pub tool_calls: u32,
    pub error: Option<String>,
    pub thread: Vec<SubagentThreadItemDto>,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::{SubagentStatusDto, SubagentTaskDto, SubagentThreadItemDto};
    use crate::modules::tasks::domain::models::subagent_task::{
        SubagentStatus, SubagentTask, SubagentThreadItem,
    };

    impl From<SubagentStatus> for SubagentStatusDto {
        fn from(status: SubagentStatus) -> Self {
            match status {
                SubagentStatus::Queued => Self::Queued,
                SubagentStatus::Running => Self::Running,
                SubagentStatus::Waiting => Self::Waiting,
                SubagentStatus::Completed => Self::Completed,
                SubagentStatus::Failed => Self::Failed,
                SubagentStatus::Cancelled => Self::Cancelled,
                SubagentStatus::TimedOut => Self::TimedOut,
                SubagentStatus::Unknown => Self::Unknown,
            }
        }
    }

    impl From<SubagentThreadItem> for SubagentThreadItemDto {
        fn from(item: SubagentThreadItem) -> Self {
            match item {
                SubagentThreadItem::Text(text) => Self::Text { text },
                SubagentThreadItem::Thinking(text) => Self::Thinking { text },
                SubagentThreadItem::Note(text) => Self::Note { text },
                SubagentThreadItem::Tool {
                    name,
                    args,
                    running,
                    is_error,
                    output,
                } => Self::Tool {
                    name,
                    args,
                    running,
                    is_error,
                    output,
                },
            }
        }
    }

    impl From<SubagentTask> for SubagentTaskDto {
        fn from(task: SubagentTask) -> Self {
            Self {
                id: task.id,
                agent: task.agent,
                label: task.label,
                prompt: task.prompt,
                status: task.status.into(),
                created_at: task.created_at,
                started_at: task.started_at,
                ended_at: task.ended_at,
                last_step: task.last_step,
                last_activity_at: task.last_activity_at,
                turns: task.turns,
                tool_calls: task.tool_calls,
                error: task.error,
                thread: task.thread.into_iter().map(Into::into).collect(),
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        fn domain_task(status: SubagentStatus) -> SubagentTask {
            SubagentTask {
                id: "t1".to_string(),
                agent: "explore".to_string(),
                label: Some("Mapa".to_string()),
                prompt: None,
                status,
                created_at: Some(1),
                started_at: Some(2),
                ended_at: None,
                last_step: Some("leyendo".to_string()),
                last_activity_at: Some(3),
                turns: 2,
                tool_calls: 5,
                error: None,
                thread: vec![
                    SubagentThreadItem::Text("hola".to_string()),
                    SubagentThreadItem::Thinking("pienso".to_string()),
                    SubagentThreadItem::Note("nota".to_string()),
                    SubagentThreadItem::Tool {
                        name: "read".to_string(),
                        args: Some("{}".to_string()),
                        running: true,
                        is_error: false,
                        output: None,
                    },
                ],
            }
        }

        #[test]
        fn maps_every_field_of_a_task() {
            let dto: SubagentTaskDto = domain_task(SubagentStatus::Running).into();

            assert_eq!(dto.id, "t1");
            assert_eq!(dto.agent, "explore");
            assert_eq!(dto.label.as_deref(), Some("Mapa"));
            assert_eq!(dto.prompt, None);
            assert_eq!(dto.status, SubagentStatusDto::Running);
            assert_eq!(
                (dto.created_at, dto.started_at, dto.ended_at),
                (Some(1), Some(2), None)
            );
            assert_eq!(dto.last_step.as_deref(), Some("leyendo"));
            assert_eq!(dto.last_activity_at, Some(3));
            assert_eq!((dto.turns, dto.tool_calls), (2, 5));
            assert_eq!(dto.error, None);
            assert_eq!(
                dto.thread,
                vec![
                    SubagentThreadItemDto::Text {
                        text: "hola".to_string()
                    },
                    SubagentThreadItemDto::Thinking {
                        text: "pienso".to_string()
                    },
                    SubagentThreadItemDto::Note {
                        text: "nota".to_string()
                    },
                    SubagentThreadItemDto::Tool {
                        name: "read".to_string(),
                        args: Some("{}".to_string()),
                        running: true,
                        is_error: false,
                        output: None,
                    },
                ]
            );
        }

        #[test]
        fn maps_every_status() {
            for (status, expected) in [
                (SubagentStatus::Queued, SubagentStatusDto::Queued),
                (SubagentStatus::Running, SubagentStatusDto::Running),
                (SubagentStatus::Waiting, SubagentStatusDto::Waiting),
                (SubagentStatus::Completed, SubagentStatusDto::Completed),
                (SubagentStatus::Failed, SubagentStatusDto::Failed),
                (SubagentStatus::Cancelled, SubagentStatusDto::Cancelled),
                (SubagentStatus::TimedOut, SubagentStatusDto::TimedOut),
                (SubagentStatus::Unknown, SubagentStatusDto::Unknown),
            ] {
                assert_eq!(SubagentStatusDto::from(status), expected);
            }
        }

        #[test]
        fn dto_survives_a_json_round_trip() {
            let dto: SubagentTaskDto = domain_task(SubagentStatus::TimedOut).into();

            let json: String = serde_json::to_string(&dto).expect("serializa");
            let back: SubagentTaskDto = serde_json::from_str(&json).expect("deserializa");

            assert!(json.contains("\"timed_out\""));
            assert!(json.contains("\"kind\":\"tool\""));
            assert_eq!(back, dto);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_queued_running_and_waiting_are_active() {
        assert!(SubagentStatusDto::Queued.is_active());
        assert!(SubagentStatusDto::Running.is_active());
        assert!(SubagentStatusDto::Waiting.is_active());
        assert!(!SubagentStatusDto::Completed.is_active());
        assert!(!SubagentStatusDto::TimedOut.is_active());
        assert!(!SubagentStatusDto::Unknown.is_active());
    }
}
