use std::fmt;

/// What iworkflow has done with an imported issue; independent from the Jira status.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum IssueLocalState {
    #[default]
    New,
    Converted,
    Ignored,
}

impl IssueLocalState {
    pub fn as_str(&self) -> &'static str {
        match self {
            IssueLocalState::New => "new",
            IssueLocalState::Converted => "converted",
            IssueLocalState::Ignored => "ignored",
        }
    }

    /// Unknown values fall back to `New` so a hand-edited row never hides an issue.
    pub fn parse(value: &str) -> Self {
        match value {
            "converted" => IssueLocalState::Converted,
            "ignored" => IssueLocalState::Ignored,
            _ => IssueLocalState::New,
        }
    }
}

impl fmt::Display for IssueLocalState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_round_trips_every_state() {
        for state in [
            IssueLocalState::New,
            IssueLocalState::Converted,
            IssueLocalState::Ignored,
        ] {
            assert_eq!(IssueLocalState::parse(state.as_str()), state);
        }
    }

    #[test]
    fn unknown_value_falls_back_to_new() {
        assert_eq!(IssueLocalState::parse("???"), IssueLocalState::New);
    }
}
