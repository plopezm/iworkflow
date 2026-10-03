# agent-profile-selector-with-pi

Goal: keep the profile selector (and the profile pin) available when the `pi` agent is selected.

- [x] Test first: pi runtime keeps the profile pin (RED observed, then GREEN)
- [x] `PrepareLaunch` applies the pin for every runtime
- [x] `AgentSettingsBar` no longer hides the profile selector for `pi`
- [x] Full verification via gentle-ai-verify: fmt, clippy ssr+hydrate, cargo test --features ssr (390), cargo leptos test (ssr 322 + wasm 54) OK
- [x] CodeGraph synced
- [x] Work-unit commit
- [x] Fixed pre-existing hydrate compile error: task_edit_state tests gated with cfg(all(test, feature = "ssr"))
