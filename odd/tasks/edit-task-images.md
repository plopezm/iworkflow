# edit-task-images

Goal: allow attaching images when editing a Backlog task (edit page reached from the board).
New images are appended to the task's existing attachments (total <= MAX_IMAGES_PER_MESSAGE).

- [x] 1. Adapter: `insert_many` appends after existing positions (offset = max(position)+1 inside the tx) + integration test
- [x] 2. UpdateTask use case: accept `images`, validate total limit, store files + metadata, cleanup new files on failure + unit tests
- [x] 3. DTO/server fn: `UpdateTaskDto.images` (serde default) + `update_task` wiring + DI + mapping tests
- [x] 4. UI: `TaskEditState.images` + `ImagePicker`/paste in `TaskEditPage` + state tests
- [x] 5. fmt, clippy (ssr/hydrate), tests; commit
