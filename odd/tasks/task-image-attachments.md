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
- [ ] 2. Persistence of initial-prompt images (Backlog -> start)
  - Migration `task_attachments` (task_id FK cascade, id, name, mime_type, data BLOB, created_at) with up/down; regenerate schema.rs.
  - Outbound port `TaskAttachmentRepositoryPort` (insert_many, list_by_task) + Diesel adapter + integration test; mocks.
  - `CreateTask` stores images; `StartTask` loads them and sends with the initial `Prompt`.
- [ ] 3. Server functions + DTOs
  - `ImageAttachmentDto` (base64), `CreateTaskDto.images`, `send_task_message(.., images)`; mapping tests; raise body limit for these routes if needed.
- [ ] 4. UI
  - Reusable `ImagePicker` component (file input, paste, thumbnails, remove) used in `TaskModal` and `Composer`; state signals in board/detail state.
  - Transcript shows image count for user messages.
- [ ] 5. Verification: fmt, clippy ssr + hydrate, `cargo test --features ssr`.

## Evidence

(pending)
