# merged-task-diff-persistence

Goal: keep the final diff of a task viewable after it is merged (today `GetTaskDiff` rejects closed tasks and the
branch/worktree are deleted on merge). We do NOT store the patch: we store the commit range (`base_sha`, `head_sha`)
in SQLite and regenerate the diff with `git diff base_sha head_sha` from the workspace repo (all worktrees share one
object store). Merge is `--no-ff`, so the commits stay reachable from the base branch; additionally pin
`refs/iworkflow/tasks/<task_id>` -> `head_sha` before cleanup.

Decisions by user: only future tasks; already merged / discarded tasks get no diff. Discarded tasks are out of scope.
Capture is best-effort: a failed capture/pin never blocks the merge (noted in the task error text instead).

- [x] 1. Persistence: migration (`base_sha`, `head_sha` nullable on `tasks`, up/down, redo OK), `schema.rs`, `TaskRow`, `Task.diff_range: Option<DiffRange>`, `TaskRepositoryPort::record_diff_range` + adapter + integration test
- [x] 2. Git port: `WorktreePort::pin_range` (resolve merge-base + branch tip, `update-ref` pin) and `diff_range`; adapter, `mocks.rs`, integration tests (merge, branch deleted, diff regenerated)
- [ ] 3. `MergeTask`: pin before merge, record range after `Merged`, best-effort errors; unit tests
- [ ] 4. `GetTaskDiff` for merged tasks with a range (open tasks unchanged, closed without range -> clear error); `can_view_diff` + DTO/server-fn mapping tests
- [ ] 5. fmt, clippy ssr/hydrate, tests, codegraph refresh; final commit evidence
