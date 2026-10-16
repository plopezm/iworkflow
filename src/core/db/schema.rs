// @generated automatically by Diesel CLI.

diesel::table! {
    agent_settings (id) {
        id -> Nullable<Integer>,
        model_provider -> Nullable<Text>,
        model_id -> Nullable<Text>,
        profile -> Nullable<Text>,
        thinking -> Nullable<Text>,
        runtime -> Nullable<Text>,
        memory_limit_mb -> Nullable<Integer>,
        cpu_limit_percent -> Nullable<Integer>,
        limits_disabled -> Integer,
    }
}

diesel::table! {
    agents_files (id) {
        id -> Text,
        description -> Text,
        path -> Text,
        created_at -> Timestamp,
    }
}

diesel::table! {
    jira_connection (id) {
        id -> Integer,
        base_url -> Text,
        token -> Text,
        jql -> Text,
        accept_invalid_certs -> Integer,
        last_sync_at -> Nullable<Timestamp>,
        last_sync_error -> Nullable<Text>,
        updated_at -> Timestamp,
    }
}

diesel::table! {
    jira_issues (key) {
        key -> Text,
        summary -> Text,
        description -> Text,
        status -> Text,
        issue_type -> Text,
        priority -> Nullable<Text>,
        assignee -> Nullable<Text>,
        reporter -> Nullable<Text>,
        labels -> Text,
        url -> Text,
        jira_updated_at -> Timestamp,
        local_state -> Text,
        task_id -> Nullable<Text>,
        synced_at -> Timestamp,
    }
}

diesel::table! {
    task_attachments (id) {
        id -> Text,
        task_id -> Text,
        position -> Integer,
        name -> Text,
        mime_type -> Text,
        path -> Text,
        size_bytes -> Integer,
        created_at -> Timestamp,
    }
}

diesel::table! {
    task_events (seq) {
        seq -> Integer,
        task_id -> Text,
        payload -> Text,
        created_at -> Timestamp,
    }
}

diesel::table! {
    tasks (id) {
        id -> Text,
        workspace_id -> Text,
        title -> Text,
        prompt -> Text,
        branch -> Text,
        base_branch -> Text,
        worktree_path -> Text,
        status -> Text,
        error -> Nullable<Text>,
        created_at -> Timestamp,
        updated_at -> Timestamp,
        pending_dialogs -> Text,
        subagents_active -> Integer,
        subagents_waiting -> Integer,
        priority -> Text,
        base_sha -> Nullable<Text>,
        head_sha -> Nullable<Text>,
        memory_limit_mb -> Nullable<Integer>,
        cpu_limit_percent -> Nullable<Integer>,
    }
}

diesel::table! {
    workspaces (id) {
        id -> Text,
        name -> Text,
        path -> Text,
        base_branch -> Text,
        created_at -> Timestamp,
    }
}

diesel::joinable!(task_attachments -> tasks (task_id));
diesel::joinable!(task_events -> tasks (task_id));
diesel::joinable!(tasks -> workspaces (workspace_id));

diesel::allow_tables_to_appear_in_same_query!(
    agent_settings,
    agents_files,
    jira_connection,
    jira_issues,
    task_attachments,
    task_events,
    tasks,
    workspaces,
);
