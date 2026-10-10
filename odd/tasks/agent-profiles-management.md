# agent-profiles-management

Goal: web screens equivalent to gentle-pi `/gentle:profiles` and `/gentle:models`: create / duplicate / rename / delete / import / export profiles, edit per-agent model + thinking, create new agent roles. Profiles created here show up in the existing profile dropdown (AgentSettingsBar reads the same `profiles.json`).

Decisions (user): web pages `/profiles` and `/models` mirroring the commands (no command palette); no global "apply" (only manage + pick in dropdown; `/models` edits `models.json` only); new roles = `<role>.md` agent file in `~/.pi/agent/agents`.

Design: new vertical slice `src/modules/agent_profiles/` (no imports from `agent_settings`); reads/writes `<gentle_pi_config_home>/profiles.json` (atomic tmp+rename, shape `gentle-pi.agent_model_profiles` v1), `models.json`, export envelope `gentle-pi.agent_model_profile` v1, agents dir `~/.pi/agent/agents`. Model options come through an outbound port implemented in the composition root (`app_state.rs`) by adapting `agent_settings::ListModels`.

- [x] T1 Domain: models (ProfileName, RoutingEntry, Profile, ProfileStore, AgentRole), errors, outbound ports, `mocks.rs`
- [x] T2 Use cases + unit tests (list, create, duplicate, rename, delete, set routing, export, import, create role, get/set global models)
- [x] T3 File adapters (profiles store, global models.json, agents dir) + integration tests (tempfile)
- [x] T3b Review follow-ups (non-blocking WARNINGs from native review of T1-T3, approved & acknowledged): role list must skip unreadable/bad entries instead of failing; role create must not leave a partial file on write failure; exchange test must not depend on ~/.pi (use fixture); lost-update on profiles.json read-modify-write (serialize writes with a mutex in the store adapter)
- [x] T4 DTOs + server fns + `agent_profiles_context.rs` + app_state/config wiring + mapping tests
- [x] T5 Presentation: state + `/profiles` page (CRUD, import/export, per-agent editor, new role) + nav link
- [x] T6 Presentation: `/models` page (global models.json editor)
- [x] T7 Full verification (fmt, clippy ssr+hydrate, tests), README note
- T1 commit: d843bae (20 unit tests, clippy ssr ok; RED not observed, tests written alongside value objects)
- T2 commit: e818300 (54 unit tests, RED observed 34 failed -> GREEN, clippy ssr all-targets ok)
- T3 commit: 50afd0e (lib 74 + 20 integration tests, RED observed E0583, clippy ssr all-targets ok)
- T3b commit: 1c714de (lib 77, 4 integration bins ok, RED observed for lost update + role list; clippy ssr all-targets ok). T4 must build mutating use cases with WriteGuard::with_guard and one shared guard
- T4 commit: 88de680 (lib 89, config 11, clippy ssr all-targets + hydrate wasm ok; RED only for config default). Note: NewAgentRoleDto.model/thinking ignored by create_agent_role (domain NewAgentRole has none)
- Review T3b+T4 (approved, acknowledged). Non-blocking follow-ups for T7: role list silently truncates when dir iteration fails midway (agent_role_dir_adapter.rs ~l.168); NewAgentRoleDto model/thinking are dropped (decide: remove from DTO or persist in frontmatter)
- T5 commit: 7eb6e31 (lib 109, RED 12 failed -> GREEN 14, clippy ssr + hydrate ok, cargo leptos build ok). NOT verified in a browser: render/hydration/styling, file input, blob download, clipboard
- T6 commit: 19fc5fd (lib 112, RED E0425 -> GREEN, clippy ssr + hydrate ok). Not verified in a browser
- Review T5+T6 (approved, acknowledged). Non-blocking follow-ups: models_page.rs:42 unsaved editor draft is lost when a role is created (resource refetch rebuilds editor); profiles_page.rs:134 selected-profile tracking after await (rename/delete races)
- T7 verification: fmt ok; clippy ssr all-targets + hydrate wasm ok; cargo test --features ssr: lib 790 + 14 integration bins ok; cargo leptos test 162/163 - terminal_key_bar::rendered_bar_has_one_button_per_key FAILS under hydrate, also on base 485f19f (pre-existing, unrelated; passes under ssr). Smoke test against the real binary with temp dirs: /profiles & /models 200, all 14 server fns round-trip, dropdown source list_profile_options returns new profiles. NOT verified: browser rendering/hydration/styling, file input, blob download, clipboard.
- Open follow-ups (non-blocking, from reviews): role list truncation on dir iteration error; NewAgentRoleDto model/thinking dropped; models_page draft lost on role create; profiles_page selection tracking after await.
- Follow-up fix commit 9bf10ec: role list now errors on dir-iteration failure (file-level skips kept); NewAgentRole/DTO model+thinking persisted in frontmatter and read back. lib 117, role adapter integration 8, clippy ssr+hydrate ok. RED seen for 4 lib tests; the 2 new adapter integration tests had no RED run. Still open: models_page draft lost on role create; profiles_page selection tracking after await.
- Review of role follow-up fix (approved, acknowledged). New non-blocking WARNING: agent_role_dir_adapter.rs:124-129 render_role does not validate thinking at the adapter (only domain NewAgentRole::validate does).
- UI follow-up fix commit 3d18558: RoutingEditor catalog props are Signals so role creation no longer rebuilds the editor (draft kept); next_selection pure fn for create/duplicate/import/rename/delete with stale-action guard. lib 132 (RED: 10 failed -> GREEN), clippy ssr+hydrate ok. Draft retention reasoned, NOT verified in a browser. All listed follow-ups closed except adapter-level thinking validation (render_role).
- Review of UI follow-up fix (approved, acknowledged). Non-blocking: profiles_state.rs:432-433 select() bump of user_picks counter not unit-proved (WARNING); routing_editor.rs:239 thinking select uses row's initial value rather than live value when catalog loads late (SUGGESTION).
