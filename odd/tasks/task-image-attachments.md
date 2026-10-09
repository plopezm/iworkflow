# Feature: task-image-attachments

Attach images to tasks; they are sent to the agent via pi RPC (`images: [{type:"image", data:<base64>, mimeType}]` on `prompt` / `steer` / `follow_up`) together with the text.
Branch: `pi/adjuntar-fotos-b1a64f4c-8bcd-4685-8bff-cb6baa7c2fae`.

Limits (domain constants): max 5 images per message, 5 MiB each, mime in png/jpeg/gif/webp.

## Tasks

- [x] 1. Domain + RPC protocol
  - `ImageAttachment { name, mime_type, data (base64 String) }` value object with validation (`TaskError::InvalidAttachment`).
  - `AgentCommand::{Prompt,Steer,FollowUp}` gain `images: Vec<ImageAttachment>`; `rpc_protocol::encode` emits `images` only when non-empty; `UserMessage` event carries `images: u32` (`#[serde(default)]`) decoded from user content image blocks; DTO + transcript show "N imagen(es)".
  - `SendMessage::execute` takes images (text or images required; text still trimmed).
  - Tests: encode/decode, validation, SendMessage with images.
- [x] 2. Persistence of initial-prompt images (Backlog -> start)
  - Migration `task_attachments` (task_id FK cascade, id, name, mime_type, data TEXT base64, created_at) with up/down; regenerate schema.rs.
  - Outbound port `TaskAttachmentRepositoryPort` (insert_many, list_by_task) + Diesel adapter + integration test; mocks.
  - `CreateTask` stores images; `StartTask` loads them and sends with the initial `Prompt`.
- [x] 3. Server functions + DTOs
  - `ImageAttachmentDto` (base64), `CreateTaskDto.images`, `send_task_message(.., images)`; mapping tests; raise body limit for these routes if needed.
- [x] 4. UI
  - Reusable `ImagePicker` component (file input, paste, thumbnails, remove) used in `TaskModal` and `Composer`; state signals in board/detail state.
  - Transcript shows image count for user messages.
- [x] 5. Verification: fmt, clippy ssr + hydrate, `cargo test --features ssr`.

## Evidence

- Task 1: `600f899` — 274 lib tasks tests, runtime adapter tests, fmt + clippy ssr/hydrate ok. RED not observed (tests written with implementation).
- Task 2: `47efd80` — compile-failure RED observed; full `cargo test --features ssr` green (incl. 6 adapter integration tests), migration redo ok, fmt + clippy ssr/hydrate ok. Attachment writes bypass RetryingTaskRepository (Busy on lock, CreateTask rolls back).
- Task 3: `16893d9` — RED (compile) observed; body-limit tests against a real axum server (413 without layer, 200 at 35 MiB, 413 over 40 MiB); full ssr suite, fmt, clippy ssr/hydrate ok. Not tested: full leptos server-fn request, main.rs wiring.
- Task 4: `d858b85` — RED not observed; unit tests for validation/DTO building; cargo leptos build ok. Not tested in a browser (picker, paste, mobile camera/gallery); no e2e harness for modal/upload.
- Task 5: independent verifier: fmt, clippy ssr+hydrate ok, cargo test --features ssr 627 passed 0 failed; layer purity grep clean; no new non-test unwrap/expect. Not run: cargo leptos test, e2e.

## Follow-up: store images on disk (user decision)

Replace base64 TEXT in SQLite with files on disk; SQLite keeps only a relative path. Max image size 4 MiB. Migration `2026-10-09-120000-0000_create_task_attachments` is rewritten in place (unmerged, not applied locally).

- [ ] 6. File-backed attachments
  - Outbound port `AttachmentStoragePort` (save/read/delete_task_dir) + FS adapter under `~/.iworkflow/attachments/<task_id>/<attachment_id>.<ext>` (`IWF_ATTACHMENTS_DIR` override, resolved in `core/config.rs` like `agents-files`); path stored relative.
  - Table keeps `id, task_id, position, name, mime_type, path, size_bytes, created_at` (no `data`); `list_by_task` returns metadata, `StartTask` reads bytes via the storage port to build `ImageAttachment`s.
  - Files removed when a task/workspace tasks are deleted; save-then-insert with file cleanup on failure; `CreateTask` rollback also deletes files.
  - Limit 4 MiB in domain, client state, hints, tests; `SERVER_FN_MAX_BODY_BYTES` -> 30 MiB.
- [ ] 7. Verification + native review
