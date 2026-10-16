/// iworkflow priority suggested from the Jira priority name; the user can change it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum PriorityHint {
    Low,
    #[default]
    Normal,
    High,
    Urgent,
}

impl PriorityHint {
    pub fn as_str(&self) -> &'static str {
        match self {
            PriorityHint::Low => "low",
            PriorityHint::Normal => "normal",
            PriorityHint::High => "high",
            PriorityHint::Urgent => "urgent",
        }
    }

    pub fn parse(value: &str) -> Self {
        match value {
            "low" => PriorityHint::Low,
            "high" => PriorityHint::High,
            "urgent" => PriorityHint::Urgent,
            _ => PriorityHint::Normal,
        }
    }

    /// Maps the default Jira priority names (Blocker, Critical, Highest, High, Major, Medium,
    /// Minor, Low, Lowest, Trivial) case-insensitively; anything else is `Normal`.
    pub fn from_jira_priority(name: Option<&str>) -> Self {
        let Some(name) = name else {
            return PriorityHint::Normal;
        };
        let lower: String = name.trim().to_lowercase();
        let matches = |words: &[&str]| words.iter().any(|w: &&str| lower.contains(w));
        if matches(&["blocker", "critical", "highest", "urgent"]) {
            PriorityHint::Urgent
        } else if matches(&["high", "major"]) {
            PriorityHint::High
        } else if matches(&["low", "minor", "trivial"]) {
            PriorityHint::Low
        } else {
            PriorityHint::Normal
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn maps_default_jira_priorities() {
        let cases: [(&str, PriorityHint); 9] = [
            ("Blocker", PriorityHint::Urgent),
            ("Critical", PriorityHint::Urgent),
            ("Highest", PriorityHint::Urgent),
            ("High", PriorityHint::High),
            ("Major", PriorityHint::High),
            ("Medium", PriorityHint::Normal),
            ("Minor", PriorityHint::Low),
            ("Lowest", PriorityHint::Low),
            ("Trivial", PriorityHint::Low),
        ];
        for (name, expected) in cases {
            assert_eq!(
                PriorityHint::from_jira_priority(Some(name)),
                expected,
                "{name}"
            );
        }
    }

    #[test]
    fn missing_or_unknown_priority_is_normal() {
        assert_eq!(PriorityHint::from_jira_priority(None), PriorityHint::Normal);
        assert_eq!(
            PriorityHint::from_jira_priority(Some("Whatever")),
            PriorityHint::Normal
        );
    }

    #[test]
    fn parse_round_trips() {
        for p in [
            PriorityHint::Low,
            PriorityHint::Normal,
            PriorityHint::High,
            PriorityHint::Urgent,
        ] {
            assert_eq!(PriorityHint::parse(p.as_str()), p);
        }
    }
}
