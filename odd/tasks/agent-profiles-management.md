# agent-profiles-management

Goal: web screens equivalent to gentle-pi `/gentle:profiles` and `/gentle:models`: create / duplicate / rename / delete / import / export profiles, edit per-agent model + thinking, create new agent roles. Profiles created here show up in the existing profile dropdown (AgentSettingsBar reads the same `profiles.json`).

Decisions (user): web pages `/profiles` and `/models` mirroring the commands (no command palette); no global "apply" (only manage + pick in dropdown; `/models` edits `models.json` only); new roles = `<role>.md` agent file in `~/.pi/agent/agents`.

Design: new vertical slice `src/modules/agent_profiles/` (no imports from `agent_settings`); reads/writes `<gentle_pi_config_home>/profiles.json` (atomic tmp+rename, shape `gentle-pi.agent_model_profiles` v1), `models.json`, export envelope `gentle-pi.agent_model_profile` v1, agents dir `~/.pi/agent/agents`. Model options come through an outbound port implemented in the composition root (`app_state.rs`) by adapting `agent_settings::ListModels`.

- [x] T1 Domain: models (ProfileName, RoutingEntry, Profile, ProfileStore, AgentRole), errors, outbound ports, `mocks.rs`
- [x] T2 Use cases + unit tests (list, create, duplicate, rename, delete, set routing, export, import, create role, get/set global models)
- [x] T3 File adapters (profiles store, global models.json, agents dir) + integration tests (tempfile)
- [x] T3b Review follow-ups (non-blocking WARNINGs from native review of T1-T3, approved & acknowledged): role list must skip unreadable/bad entries instead of failing; role create must not leave a partial file on write failure; exchange test must not depend on ~/.pi (use fixture); lost-update on profiles.json read-modify-write (serialize writes with a mutex in the store adapter)
- [x] T4 DTOs + server fns + `agent_profiles_context.rs` + app_state/config wiring + mapping tests
- [ ] T5 Presentation: state + `/profiles` page (CRUD, import/export, per-agent editor, new role) + nav link
- [ ] T6 Presentation: `/models` page (global models.json editor)
- [ ] T7 Full verification (fmt, clippy ssr+hydrate, tests), README note
- T1 commit: d843bae (20 unit tests, clippy ssr ok; RED not observed, tests written alongside value objects)
- T2 commit: e818300 (54 unit tests, RED observed 34 failed -> GREEN, clippy ssr all-targets ok)
- T3 commit: 50afd0e (lib 74 + 20 integration tests, RED observed E0583, clippy ssr all-targets ok)
- T3b commit: 1c714de (lib 77, 4 integration bins ok, RED observed for lost update + role list; clippy ssr all-targets ok). T4 must build mutating use cases with WriteGuard::with_guard and one shared guard
- T4 commit: 88de680 (lib 89, config 11, clippy ssr all-targets + hydrate wasm ok; RED only for config default). Note: NewAgentRoleDto.model/thinking ignored by create_agent_role (domain NewAgentRole has none)
- Review T3b+T4 (approved, acknowledged). Non-blocking follow-ups for T7: role list silently truncates when dir iteration fails midway (agent_role_dir_adapter.rs ~l.168); NewAgentRoleDto model/thinking are dropped (decide: remove from DTO or persist in frontmatter)
