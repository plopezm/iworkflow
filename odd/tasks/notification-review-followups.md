# notification-review-followups

Goal: close the two advisory findings of review-2f9942b8e4a34d8f on the browser notifications feature.

- [x] Test first: pure `plan_notifications(previous, tasks)` returning notifications + next baseline (first snapshot seeds without notifying; baseline always advances) — finding R3-baseline-wiring-untested (app_sidebar_state.rs:143-150)
- [x] Effect in `AppSidebarState` delegates to the pure function
- [x] `request_permission` always reports an outcome when the browser call fails — finding R3-request-permission-error-path (browser_notification.rs:74-77)
- [x] Verification via gentle-ai-verify: fmt, clippy ssr+hydrate, cargo test --features ssr, cargo leptos test
- [x] CodeGraph synced
- [x] Work-unit commit
