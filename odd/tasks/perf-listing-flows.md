# perf-listing-flows

Goal: speed up listing flows and DB writes after the audit (follow-up of perf-task-queries).
Decisions: all technical items in scope; PRAGMA synchronous=NORMAL; SSE replay incremental via Last-Event-ID (full history on first open); no task_events retention changes.

- [ ] 1. Pool pragmas: synchronous=NORMAL (+ temp_store=MEMORY) with test
- [ ] 2. Queries: list_active via eq_any(active statuses) to use (status,created_at) index; delete_inactive_by_workspace selects only id/status
- [ ] 3. SSE: stream_task_events honors Last-Event-ID (event id = seq) and replays only after it; full history only on first connect
- [ ] 4. UI: shared task store + single task_events subscription in shell; 'changed' patches one task via get_task, full reload only on reset/lagged/fallback; board + sidebar consume it
- [ ] 5. Diff: avoid git diff per updated_at bump (key on HEAD sha; cache merged base/head)
- [ ] 6. Attachments: list returns metadata only; images served by an endpoint (cacheable) instead of base64
- [ ] 7. Verify: fmt, clippy ssr/hydrate, cargo test --features ssr, cargo leptos build
