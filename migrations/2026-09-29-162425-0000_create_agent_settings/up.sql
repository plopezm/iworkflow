CREATE TABLE agent_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    model_provider TEXT NULL,
    model_id TEXT NULL,
    profile TEXT NULL
);
