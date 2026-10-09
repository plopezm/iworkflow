// @generated automatically by Diesel CLI.

diesel::table! {
    agent_settings (id) {
        id -> Integer,
        model_provider -> Nullable<Text>,
        model_id -> Nullable<Text>,
        profile -> Nullable<Text>,
        thinking -> Nullable<Text>,
        runtime -> Nullable<Text>,
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
    task_attachments (id) {
        id -> Text,
        task_id -> Text,
        position -> Integer,
        name -> Text,
        mime_type -> Text,
        data -> Text,
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
    task_attachments,
    task_events,
    tasks,
    workspaces,
);
