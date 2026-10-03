use std::fmt;

use crate::modules::tasks::domain::errors::task_error::TaskError;

/// How urgent a task is. Higher priorities are listed above lower ones.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum TaskPriority {
    Low,
    #[default]
    Normal,
    High,
    Urgent,
}

impl TaskPriority {
    pub const ALL: [TaskPriority; 4] = [
        TaskPriority::Low,
        TaskPriority::Normal,
        TaskPriority::High,
        TaskPriority::Urgent,
    ];

    pub fn as_str(&self) -> &'static str {
        match self {
            TaskPriority::Low => "low",
            TaskPriority::Normal => "normal",
            TaskPriority::High => "high",
            TaskPriority::Urgent => "urgent",
        }
    }

    pub fn parse(value: &str) -> Result<Self, TaskError> {
        Self::ALL
            .into_iter()
            .find(|p: &TaskPriority| p.as_str() == value)
            .ok_or_else(|| TaskError::InvalidPriority(value.to_string()))
    }

    /// Sorting rank: the higher the rank, the more urgent the task.
    pub fn rank(&self) -> u8 {
        match self {
            TaskPriority::Low => 0,
            TaskPriority::Normal => 1,
            TaskPriority::High => 2,
            TaskPriority::Urgent => 3,
        }
    }
}

impl fmt::Display for TaskPriority {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_through_string() {
        for priority in TaskPriority::ALL {
            assert_eq!(TaskPriority::parse(priority.as_str()), Ok(priority));
        }
    }

    #[test]
    fn rejects_unknown_value() {
        assert_eq!(
            TaskPriority::parse("critical"),
            Err(TaskError::InvalidPriority("critical".to_string()))
        );
    }

    #[test]
    fn defaults_to_normal() {
        assert_eq!(TaskPriority::default(), TaskPriority::Normal);
    }

    #[test]
    fn rank_increases_with_urgency() {
        let ranks: Vec<u8> = TaskPriority::ALL
            .iter()
            .map(|p: &TaskPriority| p.rank())
            .collect();
        assert!(ranks.windows(2).all(|pair: &[u8]| pair[0] < pair[1]));
    }
}
