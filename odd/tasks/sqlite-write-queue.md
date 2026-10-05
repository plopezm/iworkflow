# sqlite-write-queue

Goal: no agent event is ever lost to `database is locked`. Scope (user-approved): per-task ordered
event queue + per-call retry with backoff, inside the `tasks` slice. Out of scope: global single
writer, on-disk spool (a process crash with queued events can still lose them; documented).

## Design
- `TaskError::Busy` distinguishes transient SQLite lock errors from other persistence errors.
- `RetryingTaskRepository` (decorator over `TaskRepositoryPort`): retries only `Busy`, per call
  (each call is atomic, so retry is idempotent-safe; never retry whole use cases: `record` would
  duplicate events). Exponential backoff with jitter-free cap; policy has optional max elapsed.
- `QueuedAgentEventSink` (implements `AgentEventSink`): ONE bounded mpsc + single FIFO worker (per-task
  workers rejected: idle-exit races can lose/reorder events). `handle` returns once enqueued
  (backpressure only when full). Never drops; per-event panic isolation; inline fallback if worker gone.
- DI: events path uses an unbounded-retry policy; UI/request path a bounded one.

## Tasks
- [ ] T0 Commit existing fix: `immediate_transaction` + concurrency tests (branch `fix/sqlite-write-queue`)
- [x] T1 `TaskError::Busy` + adapter `db_error` mapping + tests
- [x] T2 `RetryingTaskRepository` decorator + policy + unit tests (mockall, paused time)
- [x] T3 `QueuedAgentEventSink` + unit tests (order, no loss, backpressure, worker lifecycle)
- [x] T4 DI wiring in `tasks_context.rs` + integration test: external connection holds write lock past `busy_timeout`, all events persisted in order after release
- [x] T5 Final checks (fmt, clippy ssr+hydrate, tests), codegraph sync, docs note

## Evidence
- T0: 9cbe634
- T1: 2aa655e (fmt, clippy ssr+hydrate, 360 lib tests + integration ok)
- T2: ee01c1c (7 tests; mutation-checked: no-retry, no-cap, retry-nonTransient all killed)
- T3: c92815c (7 tests; mutation-checked: try_send, no-op handle, no panic isolation all killed)
- T4: 78bea3c (integration test 6.6s; mutants killed: plain wiring blocks reader, queue-without-retry loses dialog-0)
- T5: fmt ok; clippy ssr --all-targets and hydrate wasm32 -D warnings clean; cargo test --features ssr: 446 passed, 0 failed; codegraph sync done.

## Known limits
- Events still in memory are lost if the process is killed. Queue.shutdown() exists but main.rs has no graceful-shutdown hook yet.
- Not run: cargo leptos test, e2e.
