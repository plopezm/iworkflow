//! Pure mapping from Jira REST v2 JSON documents to domain models.

use chrono::{DateTime, Utc};
use serde_json::Value;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_comment::JiraComment;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::RemoteIssue;
use crate::modules::jira::domain::models::jira_transition::JiraTransition;

/// Fields requested from Jira; keeps responses small.
pub const ISSUE_FIELDS: &str =
    "summary,description,status,issuetype,priority,assignee,reporter,labels,updated";

fn text(value: &Value, pointer: &str) -> Option<String> {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .map(str::to_string)
}

/// Jira timestamps look like `2024-05-01T10:00:00.000+0200` (no colon in the offset).
pub fn parse_jira_date(value: &str) -> Option<DateTime<Utc>> {
    DateTime::parse_from_str(value, "%Y-%m-%dT%H:%M:%S%.3f%z")
        .or_else(|_| DateTime::parse_from_rfc3339(value))
        .ok()
        .map(|date: DateTime<chrono::FixedOffset>| date.with_timezone(&Utc))
}

pub fn issue_from_json(
    connection: &JiraConnection,
    value: &Value,
) -> Result<RemoteIssue, JiraError> {
    let key: String = text(value, "/key")
        .ok_or_else(|| JiraError::Remote("respuesta de Jira sin clave de tarea".to_string()))?;
    let labels: Vec<String> = value
        .pointer("/fields/labels")
        .and_then(Value::as_array)
        .map(|all: &Vec<Value>| {
            all.iter()
                .filter_map(|label: &Value| label.as_str().map(str::to_string))
                .collect()
        })
        .unwrap_or_default();
    let person = |pointer: &str| -> Option<String> {
        text(value, &format!("{pointer}/displayName"))
            .or_else(|| text(value, &format!("{pointer}/name")))
    };
    Ok(RemoteIssue {
        url: connection.browse_url(&key),
        summary: text(value, "/fields/summary").unwrap_or_default(),
        description: text(value, "/fields/description").unwrap_or_default(),
        status: text(value, "/fields/status/name").unwrap_or_default(),
        issue_type: text(value, "/fields/issuetype/name").unwrap_or_default(),
        priority: text(value, "/fields/priority/name"),
        assignee: person("/fields/assignee"),
        reporter: person("/fields/reporter"),
        labels,
        jira_updated_at: text(value, "/fields/updated")
            .and_then(|date: String| parse_jira_date(&date))
            .unwrap_or_else(Utc::now),
        key,
    })
}

/// A page of `/search`: the issues plus `(start_at + issues, total)` to decide on the next page.
pub fn search_page_from_json(
    connection: &JiraConnection,
    value: &Value,
) -> Result<(Vec<RemoteIssue>, u64, u64), JiraError> {
    let issues: Vec<RemoteIssue> = value
        .get("issues")
        .and_then(Value::as_array)
        .ok_or_else(|| JiraError::Remote("respuesta de búsqueda inesperada".to_string()))?
        .iter()
        .map(|issue: &Value| issue_from_json(connection, issue))
        .collect::<Result<_, _>>()?;
    let start_at: u64 = value.get("startAt").and_then(Value::as_u64).unwrap_or(0);
    let total: u64 = value.get("total").and_then(Value::as_u64).unwrap_or(0);
    let next: u64 = start_at + issues.len() as u64;
    Ok((issues, next, total))
}

pub fn comments_from_json(value: &Value) -> Vec<JiraComment> {
    value
        .get("comments")
        .and_then(Value::as_array)
        .map(|all: &Vec<Value>| {
            all.iter()
                .map(|comment: &Value| JiraComment {
                    id: text(comment, "/id").unwrap_or_default(),
                    author: text(comment, "/author/displayName")
                        .or_else(|| text(comment, "/author/name"))
                        .unwrap_or_default(),
                    body: text(comment, "/body").unwrap_or_default(),
                    created_at: text(comment, "/created")
                        .and_then(|date: String| parse_jira_date(&date))
                        .unwrap_or_else(Utc::now),
                })
                .collect()
        })
        .unwrap_or_default()
}

pub fn transitions_from_json(value: &Value) -> Vec<JiraTransition> {
    value
        .get("transitions")
        .and_then(Value::as_array)
        .map(|all: &Vec<Value>| {
            all.iter()
                .filter_map(|transition: &Value| {
                    let id: String = text(transition, "/id")?;
                    let name: String = text(transition, "/name")?;
                    Some(JiraTransition {
                        to_status: text(transition, "/to/name").unwrap_or_else(|| name.clone()),
                        id,
                        name,
                    })
                })
                .collect()
        })
        .unwrap_or_default()
}

/// First message of a Jira error body (`errorMessages` or `errors`), if any.
pub fn error_message(body: &str) -> Option<String> {
    let value: Value = serde_json::from_str(body).ok()?;
    if let Some(message) = value
        .get("errorMessages")
        .and_then(Value::as_array)
        .and_then(|all: &Vec<Value>| all.first())
        .and_then(Value::as_str)
    {
        return Some(message.to_string());
    }
    value
        .get("errors")
        .and_then(Value::as_object)
        .and_then(|all| all.values().next())
        .and_then(Value::as_str)
        .map(str::to_string)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    fn connection() -> JiraConnection {
        JiraConnection::new("https://jira.local", "t", "", false).expect("valid")
    }

    #[test]
    fn parses_jira_dates_with_and_without_colon_offsets() {
        let a: DateTime<Utc> = parse_jira_date("2024-05-01T10:00:00.000+0200").expect("a");
        let b: DateTime<Utc> = parse_jira_date("2024-05-01T08:00:00.000Z").expect("b");
        assert_eq!(a, b);
        assert!(parse_jira_date("2024-05-01T10:00:00+02:00").is_some());
        assert_eq!(parse_jira_date("ayer"), None);
    }

    #[test]
    fn maps_a_full_issue() {
        let doc: Value = json!({
            "key": "ABC-1",
            "fields": {
                "summary": "Falla el login",
                "description": "h1. Pasos",
                "status": {"name": "In Progress"},
                "issuetype": {"name": "Bug"},
                "priority": {"name": "High"},
                "assignee": {"displayName": "Ana", "name": "ana"},
                "reporter": {"name": "luis"},
                "labels": ["backend", "urgente"],
                "updated": "2024-05-01T10:00:00.000+0200"
            }
        });
        let issue: RemoteIssue = issue_from_json(&connection(), &doc).expect("issue");
        assert_eq!(issue.key, "ABC-1");
        assert_eq!(issue.summary, "Falla el login");
        assert_eq!(issue.description, "h1. Pasos");
        assert_eq!(issue.status, "In Progress");
        assert_eq!(issue.issue_type, "Bug");
        assert_eq!(issue.priority.as_deref(), Some("High"));
        assert_eq!(issue.assignee.as_deref(), Some("Ana"));
        assert_eq!(issue.reporter.as_deref(), Some("luis"));
        assert_eq!(issue.labels, vec!["backend", "urgente"]);
        assert_eq!(issue.url, "https://jira.local/browse/ABC-1");
    }

    #[test]
    fn maps_an_issue_with_null_optional_fields() {
        let doc: Value = json!({
            "key": "ABC-2",
            "fields": {"summary": "x", "description": null, "assignee": null, "priority": null,
                       "status": {"name": "To Do"}, "issuetype": {"name": "Task"}}
        });
        let issue: RemoteIssue = issue_from_json(&connection(), &doc).expect("issue");
        assert_eq!(issue.description, "");
        assert_eq!(issue.assignee, None);
        assert_eq!(issue.priority, None);
        assert!(issue.labels.is_empty());
    }

    #[test]
    fn an_issue_without_key_is_an_error() {
        assert!(matches!(
            issue_from_json(&connection(), &json!({"fields": {}})),
            Err(JiraError::Remote(_))
        ));
    }

    #[test]
    fn search_page_reports_the_next_offset_and_total() {
        let doc: Value = json!({
            "startAt": 50, "total": 120,
            "issues": [{"key": "A-1", "fields": {}}, {"key": "A-2", "fields": {}}]
        });
        let (issues, next, total) = search_page_from_json(&connection(), &doc).expect("page");
        assert_eq!(issues.len(), 2);
        assert_eq!((next, total), (52, 120));
    }

    #[test]
    fn maps_comments_and_transitions() {
        let comments: Vec<JiraComment> = comments_from_json(&json!({"comments": [
            {"id": "1", "author": {"displayName": "Ana"}, "body": "hola",
             "created": "2024-05-01T10:00:00.000+0200"}
        ]}));
        assert_eq!(comments.len(), 1);
        assert_eq!(comments[0].author, "Ana");
        assert_eq!(comments[0].body, "hola");

        let transitions: Vec<JiraTransition> = transitions_from_json(&json!({"transitions": [
            {"id": "31", "name": "Cerrar", "to": {"name": "Done"}},
            {"id": "11", "name": "Reabrir"}
        ]}));
        assert_eq!(transitions[0].to_status, "Done");
        assert_eq!(transitions[1].to_status, "Reabrir");
    }

    #[test]
    fn extracts_error_messages() {
        assert_eq!(
            error_message(r#"{"errorMessages":["JQL inválido"],"errors":{}}"#).as_deref(),
            Some("JQL inválido")
        );
        assert_eq!(
            error_message(r#"{"errorMessages":[],"errors":{"jql":"mal"}}"#).as_deref(),
            Some("mal")
        );
        assert_eq!(error_message("<html>"), None);
    }
}
