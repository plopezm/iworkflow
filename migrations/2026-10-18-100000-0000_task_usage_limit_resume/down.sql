UPDATE tasks SET status = 'interrupted' WHERE status = 'rate_limited';
ALTER TABLE tasks DROP COLUMN resume_attempts;
ALTER TABLE tasks DROP COLUMN resume_at;
