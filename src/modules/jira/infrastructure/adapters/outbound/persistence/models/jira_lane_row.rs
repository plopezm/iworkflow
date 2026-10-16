use chrono::NaiveDateTime;
use diesel::prelude::*;

use crate::core::db::schema::{jira_lane_issues, jira_lanes};
use crate::modules::jira::domain::models::jira_lane::JiraLane;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = jira_lanes)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct JiraLaneRow {
    pub id: String,
    pub name: String,
    pub jql: String,
    pub is_default: i32,
    pub collapsed: i32,
    pub position: i32,
    pub created_at: NaiveDateTime,
}

#[derive(Debug, Clone, Insertable)]
#[diesel(table_name = jira_lane_issues)]
pub struct JiraLaneIssueRow {
    pub lane_id: String,
    pub issue_key: String,
}

impl From<JiraLaneRow> for JiraLane {
    fn from(row: JiraLaneRow) -> Self {
        Self {
            id: row.id,
            name: row.name,
            jql: row.jql,
            is_default: row.is_default != 0,
            collapsed: row.collapsed != 0,
        }
    }
}
