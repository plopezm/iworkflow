# simplify-browser-notifications

## Objective
Browser notifications must be only: the browser's native permission prompt + a native notification when a task finishes. Today finishing (Running/Starting -> Idle on RunSettled) never notifies, so it "does not work".

## Scope / constraints
- Follow AGENTS.md. Remove: custom "Activar notificaciones" button and permission signal, needs_attention/failed/conflict/interrupted/merged/awaiting_review notifications, inactivity gating, click-to-navigate.
- Checks: fmt, clippy ssr --tests + hydrate wasm (-D warnings), cargo test --features ssr.

## Tasks
- [x] T1 Notify on Starting/Running -> Idle only; request permission natively on first user interaction; delete the rest; update tests

## Progress / evidence
- T1 done: notify only on Starting/Running -> Idle ("Tarea terminada"); permission requested via one-shot document click listener (native prompt); button, permission signal, needs_attention/other-status notifications, inactivity gating and click-to-navigate removed.
- Checks passed: cargo fmt --all -- --check; cargo clippy --features ssr --tests -- -D warnings; cargo clippy --features hydrate --target wasm32-unknown-unknown --no-default-features -- -D warnings; cargo test --features ssr (all ok).
- Not verified: real browser prompt/notification behavior (manual check needed).
