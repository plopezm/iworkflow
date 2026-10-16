CREATE TABLE gitlab_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    base_url TEXT NOT NULL,
    token TEXT NOT NULL,
    insecure_tls INTEGER NOT NULL DEFAULT 0
);
