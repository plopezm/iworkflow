# task-attachments-remove-and-cleanup

Goal: in Create/Edit task, list and remove attached images while task is Backlog; on FinishTask success delete the task's attachments (DB rows + disk); cleanup failures are logged (warn) and never fail the flow.

- [x] 1. Repo port/adapter: `remove(task_id, ids) -> Vec<StoredAttachment>` and `delete_by_task(task_id)`; integration tests; mocks
- [x] 2. Use cases: `ListTaskAttachments` (metadata + base64), `UpdateTask` with `remove_attachment_ids` (limit counts remaining), `FinishTask` cleanup via best-effort service; unit tests
- [x] 3. DTOs, server fns (`list_task_attachments`, update DTO field), DI wiring; mapping tests
- [x] 4. Edit UI: show existing images with remove button, send removed ids; state tests
- [x] 5. fmt, clippy (ssr+hydrate), tests, codegraph sync
