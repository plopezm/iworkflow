CREATE TABLE code_host_settings (
    provider TEXT NOT NULL PRIMARY KEY CHECK (provider IN ('gitlab', 'github')),
    base_url TEXT NOT NULL,
    token TEXT NOT NULL,
    insecure_tls INTEGER NOT NULL DEFAULT 0
);

INSERT INTO code_host_settings (provider, base_url, token, insecure_tls)
SELECT 'gitlab', base_url, token, insecure_tls FROM gitlab_settings;

DROP TABLE gitlab_settings;
