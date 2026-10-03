# browser-task-notifications

Goal: raise a browser (Web Notifications API, shown by the OS) notification when a task needs attention or finishes. Client-only; no backend or migration changes.

Design: `AppSidebarState` (always mounted in the shell, already refreshed via SSE) diffs the previous task snapshot against the new one with a pure function and fires notifications for transitions. A core helper wraps `web_sys::Notification` (hydrate only). Permission is requested from a user gesture (sidebar button).

- [x] Test first: pure transition detector (`needs_attention` false->true, status -> AwaitingReview/Merged/Failed/Conflict/Interrupted; no notify on first snapshot or unchanged state)
- [x] `core/components/browser_notification.rs`: permission query/request + show (hydrate only, no-op on ssr), web-sys features added
- [x] Wire into `AppSidebarState` + "enable notifications" button in `AppSidebar`; notify only when tab hidden; click focuses the task route
- [x] Verification via gentle-ai-verify: fmt, clippy ssr+hydrate, cargo test --features ssr, cargo leptos test
- [x] CodeGraph synced
- [x] Work-unit commit
