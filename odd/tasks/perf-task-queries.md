# perf-task-queries

Goal: faster task loading. Do not load finished tasks unless shown; add indexes; paginate unbounded lists.

- [x] 1. Migration: composite indexes (tasks status+created_at, workspace_id+created_at), replacing redundant single-column ones; up/down, redo, schema.rs
- [x] 2. Port/adapter/use cases: `list_active` (non-Merged/Discarded) and `list_done_page(workspace, limit, offset)`; retry wrapper, mocks, adapter integration tests
- [x] 3. Server fns + UI: list_tasks returns active only; new list_done_tasks paged; board loads done page(s) only when show_done is on ("cargar más"); sidebar counts active
- [x] 4. Verify: fmt, clippy ssr/hydrate, tests

Findings not implemented (needs UI decision): SSE replay of full task_events history per connection.
