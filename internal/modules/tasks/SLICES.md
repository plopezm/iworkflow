# Tasks module: slice map (S6)

Owner: T11 (slice A, task-core). Source of truth for the sizes: `src/modules/tasks` and `tests/modules/tasks`
in the Rust tree, measured with line counts on `feat/golang` at the time of this writing.

The module is migrated in the order A → B → D → C → E → G → F → H (S6). Each slice lands in the same
package tree under `internal/modules/tasks/` (domain, application, infrastructure, presentation), so a
later slice adds files to the folders that already exist instead of creating a new layout.

## Slice definitions

| Slice | Name | Scope |
|---|---|---|
| A | task-core | CRUD, status and lifecycle rules, pending dialogs, subagent counters, usage-limit schedule, retries, repository, delete of a workspace's inactive tasks |
| B | events and SSE | event persistence stream, live bus, SSE endpoint, queued agent event sink |
| D | attachments and files | image attachments (metadata, files, HTTP), task file read/write |
| C | graph | dependencies, node positions, successor triggers, graph background loop |
| E | worktree and git | worktree, diff, merge, finish, reset, discard, branches, orphan sweep, remote merge bridge |
| G | usage-limit resume loop | rate-limited resume, manual resume, resume loop |
| F | agent runtime | runtimes (pi, claude, cursor), process control, start, messages, modes, reconcile |
| H | presentation | board, pages, sidebar, composer, transcript, server functions, workspace options |

## Sizes (Rust lines)

```
A src_files=53 src_lines=8092 test_files=1 test_lines=1133
B src_files=10 src_lines=1650 test_files=1 test_lines=307
C src_files=20 src_lines=4477 test_files=1 test_lines=281
D src_files=26 src_lines=2983 test_files=2 test_lines=595
E src_files=26 src_lines=5941 test_files=1 test_lines=830
F src_files=24 src_lines=9409 test_files=3 test_lines=1246
G src_files=4 src_lines=595 test_files=0 test_lines=0
H src_files=30 src_lines=9069 test_files=0 test_lines=0
UNMAPPED []
UNMAPPED_TESTS []
TOTAL_SRC 42216 FILES 193
```

Totals: 193 Rust files, 42,216 lines under `src/modules/tasks`; 9 integration test files, 4,392 lines under
`tests/modules/tasks`. By owner, slice A (8,092 lines in 53 files, shared glue included) is the third largest after F and H;
the glue is `mod.rs`, `test_support.rs`, `mocks.rs` and `tasks_context.rs`.

## Rules used for the mapping

- A file belongs to the slice whose behaviour it implements. Port contracts that a later slice implements are
  still declared in the domain by slice A (see "Go status" below).
- Shared domain models that slice A needs (`TaskEvent`, `SubagentTask`, `PendingDialog`, `ImageAttachment`) are A.
- Files that touch two slices are assigned to the one that owns their main behaviour. The ambiguous ones are listed
  under "Uncertainties".
- Remaining `presentation/**` files not named by another slice are H.

## Go status after slice A

| Area | Status |
|---|---|
| Domain models and errors (A) | ported, unit tested (`domain/models/*_test.go`) |
| Domain ports A–H | declared (interfaces), only A implementations exist |
| Consumer-side ports (WorkspaceLookup, LaunchSettings, RemoteMerge, WorktreeLayout, StartTaskUseCase) | declared in `domain/ports/outbound/consumed.go`, `application/create_task.go`, `domain/ports/inbound/contracts.go` |
| Use cases A | ported, unit tested (`application/*_test.go`) |
| TaskLifecycle | ported, unit tested |
| Persistence A (repository, retry decorator) | ported, integration + race tests (`tests/modules/tasks`, `persistence/*_test.go`) |
| DTOs A | ported, unit tested (`dto/task_dto_test.go`) |
| DI | `infrastructure/di/tasks_module.go` (`NewModule(Deps)`) |
| Handlers, templ, routes | not in slice A |

## Per-file mapping

### Slice A
- `application/mod.rs` (4)
- `application/services/mod.rs` (3)
- `application/services/task_lifecycle.rs` (323)
- `application/test_support.rs` (216)
- `application/usecases/answer_dialog.rs` (154)
- `application/usecases/create_task.rs` (681)
- `application/usecases/delete_task.rs` (136)
- `application/usecases/delete_workspace_tasks.rs` (252)
- `application/usecases/get_task.rs` (36)
- `application/usecases/get_task_subagents.rs` (108)
- `application/usecases/handle_agent_event.rs` (233)
- `application/usecases/list_done_tasks.rs` (86)
- `application/usecases/list_tasks.rs` (48)
- `application/usecases/mod.rs` (36)
- `application/usecases/update_task.rs` (780)
- `domain/errors/mod.rs` (1)
- `domain/errors/task_error.rs` (73)
- `domain/mod.rs` (3)
- `domain/models/image_attachment.rs` (123)
- `domain/models/mod.rs` (22)
- `domain/models/pending_dialog.rs` (40)
- `domain/models/resource_limits.rs` (107)
- `domain/models/subagent_activity.rs` (34)
- `domain/models/subagent_task.rs` (73)
- `domain/models/task.rs` (901)
- `domain/models/task_event.rs` (59)
- `domain/models/task_priority.rs` (88)
- `domain/models/usage_limit.rs` (147)
- `domain/models/workspace_ref.rs` (10)
- `domain/ports/inbound/mod.rs` (3)
- `domain/ports/inbound/start_task_use_case.rs` (8)
- `domain/ports/mod.rs` (2)
- `domain/ports/outbound/launch_settings_port.rs` (11)
- `domain/ports/outbound/mod.rs` (11)
- `domain/ports/outbound/task_event_publisher_port.rs` (6)
- `domain/ports/outbound/task_repository_port.rs` (106)
- `domain/ports/outbound/workspace_lookup_port.rs` (9)
- `infrastructure/adapters/inbound/dto/mod.rs` (7)
- `infrastructure/adapters/inbound/dto/task_dto.rs` (930)
- `infrastructure/adapters/inbound/mod.rs` (6)
- `infrastructure/adapters/mod.rs` (3)
- `infrastructure/adapters/outbound/mod.rs` (6)
- `infrastructure/adapters/outbound/persistence/mod.rs` (5)
- `infrastructure/adapters/outbound/persistence/models/mod.rs` (4)
- `infrastructure/adapters/outbound/persistence/models/task_event_row.rs` (59)
- `infrastructure/adapters/outbound/persistence/models/task_row.rs` (261)
- `infrastructure/adapters/outbound/persistence/retrying_task_repository.rs` (432)
- `infrastructure/adapters/outbound/persistence/task_repository_adapter.rs` (626)
- `infrastructure/di/mod.rs` (1)
- `infrastructure/di/tasks_context.rs` (567)
- `infrastructure/mod.rs` (5)
- `mocks.rs` (239)
- `mod.rs` (8)
- tests: `task_repository_adapter_test.rs` (1133)

### Slice B
- `application/services/queued_agent_event_sink.rs` (347)
- `application/usecases/get_task_events.rs` (51)
- `domain/models/agent_event.rs` (151)
- `domain/ports/inbound/agent_event_sink.rs` (9)
- `infrastructure/adapters/inbound/dto/task_event_dto.rs` (343)
- `infrastructure/adapters/inbound/sse.rs` (323)
- `infrastructure/adapters/outbound/events/broadcast_task_event_bus.rs` (72)
- `infrastructure/adapters/outbound/events/mod.rs` (1)
- `presentation/state/task_events.rs` (152)
- `presentation/state/task_notifications.rs` (201)
- tests: `event_queue_resilience_test.rs` (307)

### Slice C
- `application/usecases/add_task_dependency.rs` (197)
- `application/usecases/detach_task_from_graph.rs` (82)
- `application/usecases/get_task_graph.rs` (66)
- `application/usecases/remove_task_dependency.rs` (67)
- `application/usecases/set_task_position.rs` (52)
- `application/usecases/trigger_successors.rs` (421)
- `domain/models/task_graph.rs` (274)
- `domain/ports/inbound/successor_trigger.rs` (8)
- `domain/ports/outbound/task_graph_repository_port.rs` (31)
- `infrastructure/adapters/inbound/dto/task_graph_dto.rs` (99)
- `infrastructure/adapters/outbound/persistence/models/task_graph_row.rs` (85)
- `infrastructure/adapters/outbound/persistence/task_graph_repository_adapter.rs` (179)
- `infrastructure/background/graph_loop.rs` (81)
- `presentation/components/graph_edge.rs` (241)
- `presentation/components/task_graph_canvas.rs` (342)
- `presentation/components/task_node.rs` (312)
- `presentation/state/graph_geometry.rs` (447)
- `presentation/state/graph_interaction.rs` (394)
- `presentation/state/graph_model.rs` (928)
- `presentation/state/placement.rs` (171)
- tests: `task_graph_repository_adapter_test.rs` (281)

### Slice D
- `application/services/attachment_cleanup.rs` (80)
- `application/usecases/get_task_attachment.rs` (137)
- `application/usecases/get_task_file.rs` (99)
- `application/usecases/list_task_attachments.rs` (69)
- `application/usecases/save_task_file.rs` (163)
- `domain/models/stored_attachment.rs` (9)
- `domain/models/task_attachment_content.rs` (6)
- `domain/models/task_file_content.rs` (6)
- `domain/ports/outbound/attachment_storage_port.rs` (24)
- `domain/ports/outbound/task_attachment_repository_port.rs` (25)
- `domain/ports/outbound/task_file_port.rs` (15)
- `infrastructure/adapters/inbound/attachment_http.rs` (151)
- `infrastructure/adapters/inbound/dto/image_attachment_dto.rs` (145)
- `infrastructure/adapters/inbound/dto/task_file_dto.rs` (56)
- `infrastructure/adapters/outbound/files/fs_task_file_adapter.rs` (271)
- `infrastructure/adapters/outbound/files/mod.rs` (1)
- `infrastructure/adapters/outbound/persistence/models/task_attachment_row.rs` (69)
- `infrastructure/adapters/outbound/persistence/task_attachment_repository_adapter.rs` (145)
- `infrastructure/adapters/outbound/storage/fs_attachment_storage_adapter.rs` (132)
- `infrastructure/adapters/outbound/storage/mod.rs` (1)
- `presentation/components/diff_file_editor.rs` (205)
- `presentation/components/existing_attachments.rs` (62)
- `presentation/components/image_picker.rs` (238)
- `presentation/components/task_attachments_list.rs` (53)
- `presentation/state/image_attachments.rs` (423)
- `presentation/state/task_edit_state.rs` (398)
- tests: `fs_attachment_storage_adapter_test.rs` (225)
- tests: `task_attachment_repository_adapter_test.rs` (370)

### Slice E
- `application/usecases/discard_task.rs` (93)
- `application/usecases/finish_task.rs` (212)
- `application/usecases/get_task_diff.rs` (166)
- `application/usecases/list_workspace_branches.rs` (86)
- `application/usecases/merge_task.rs` (553)
- `application/usecases/reset_task.rs` (198)
- `application/usecases/sweep_orphan_worktrees.rs` (91)
- `domain/models/branch_option.rs` (10)
- `domain/models/worktree.rs` (26)
- `domain/ports/outbound/remote_merge_port.rs` (17)
- `domain/ports/outbound/worktree_port.rs` (88)
- `infrastructure/adapters/inbound/dto/task_diff_dto.rs` (21)
- `infrastructure/adapters/outbound/git/caching_worktree.rs` (373)
- `infrastructure/adapters/outbound/git/git_cli_worktree_adapter.rs` (733)
- `infrastructure/adapters/outbound/git/mod.rs` (2)
- `presentation/components/diff_badges.rs` (97)
- `presentation/components/diff_file_card.rs` (330)
- `presentation/components/diff_file_tree.rs` (146)
- `presentation/components/diff_report.rs` (146)
- `presentation/components/diff_view.rs` (365)
- `presentation/pages/task_diff_page.rs` (143)
- `presentation/state/diff_files.rs` (601)
- `presentation/state/diff_report.rs` (262)
- `presentation/state/diff_report_state.rs` (610)
- `presentation/state/diff_view_state.rs` (368)
- `presentation/state/task_merge.rs` (204)
- tests: `git_cli_worktree_adapter_test.rs` (830)

### Slice F
- `application/usecases/abort_task.rs` (69)
- `application/usecases/reconcile_tasks.rs` (147)
- `application/usecases/send_message.rs` (258)
- `application/usecases/set_agent_mode.rs` (97)
- `application/usecases/start_task.rs` (958)
- `domain/models/agent_capabilities.rs` (80)
- `domain/models/agent_command.rs` (36)
- `domain/models/agent_mode.rs` (39)
- `domain/models/launch_options.rs` (123)
- `domain/models/runtime_id.rs` (58)
- `domain/ports/outbound/agent_runtime_port.rs` (61)
- `infrastructure/adapters/inbound/dto/subagent_task_dto.rs` (245)
- `infrastructure/adapters/outbound/runtime/agent_process.rs` (957)
- `infrastructure/adapters/outbound/runtime/agent_runtime_router.rs` (667)
- `infrastructure/adapters/outbound/runtime/claude/claude_stream_runtime_adapter.rs` (242)
- `infrastructure/adapters/outbound/runtime/claude/mod.rs` (2)
- `infrastructure/adapters/outbound/runtime/claude/stream_protocol.rs` (1630)
- `infrastructure/adapters/outbound/runtime/cursor/acp_protocol.rs` (2148)
- `infrastructure/adapters/outbound/runtime/cursor/cursor_acp_runtime_adapter.rs` (191)
- `infrastructure/adapters/outbound/runtime/cursor/mod.rs` (2)
- `infrastructure/adapters/outbound/runtime/mod.rs` (5)
- `infrastructure/adapters/outbound/runtime/pi/mod.rs` (2)
- `infrastructure/adapters/outbound/runtime/pi/pi_rpc_runtime_adapter.rs` (232)
- `infrastructure/adapters/outbound/runtime/pi/rpc_protocol.rs` (1160)
- tests: `claude_stream_runtime_adapter_test.rs` (464)
- tests: `cursor_acp_runtime_adapter_test.rs` (310)
- tests: `pi_rpc_runtime_adapter_test.rs` (472)

### Slice G
- `application/usecases/resume_rate_limited_tasks.rs` (282)
- `application/usecases/resume_task.rs` (287)
- `infrastructure/background/mod.rs` (2)
- `infrastructure/background/resume_loop.rs` (24)

### Slice H
- `application/usecases/list_workspace_options.rs` (42)
- `infrastructure/adapters/inbound/server_fns.rs` (457)
- `presentation/components/app_sidebar.rs` (202)
- `presentation/components/board_toolbar.rs` (79)
- `presentation/components/composer.rs` (374)
- `presentation/components/dialog_panel.rs` (140)
- `presentation/components/loose_task_card.rs` (146)
- `presentation/components/loose_tasks_panel.rs` (238)
- `presentation/components/mod.rs` (26)
- `presentation/components/stage_badge.rs` (41)
- `presentation/components/subagents_panel.rs` (376)
- `presentation/components/task_actions.rs` (391)
- `presentation/components/task_card.rs` (326)
- `presentation/components/task_modal.rs` (462)
- `presentation/components/task_status_badge.rs` (9)
- `presentation/components/transcript_view.rs` (413)
- `presentation/components/workspace_switcher.rs` (79)
- `presentation/mod.rs` (3)
- `presentation/pages/board_page.rs` (479)
- `presentation/pages/mod.rs` (4)
- `presentation/pages/task_detail_page.rs` (863)
- `presentation/pages/task_edit_page.rs` (291)
- `presentation/state/app_sidebar_state.rs` (273)
- `presentation/state/board_state.rs` (1469)
- `presentation/state/composer_completion.rs` (306)
- `presentation/state/loose_drag.rs` (161)
- `presentation/state/mod.rs` (20)
- `presentation/state/task_detail_state.rs` (603)
- `presentation/state/task_store.rs` (251)
- `presentation/state/transcript.rs` (545)


## Uncertainties from the exploration

- `start_task.rs` (958 lines) is mapped to F because the agent launch dominates it, but it creates the worktree
  (E) and reads the launch settings. Its Go port needs E's worktree port and F's runtime port.
- `resume_task.rs` is mapped to G; it is also the manual resume button, so H consumes it. Ownership may move to F.
- `reconcile_tasks.rs` is mapped to F; it repairs the status of tasks after a restart, which needs the runtime.
- `delete_workspace_tasks.rs` is mapped to A because the cascade is part of workspace deletion. Its worktree and
  branch removal depends on E. Slice A implements the rows, the attachment folders and the removal notices.
- `reset_task.rs`, `discard_task.rs` and `finish_task.rs` need worktree removal, so they are E even though they
  change status.
- `task_event_row.rs` is mapped to A because the repository writes events; the SSE and bus code is B.
- `diff_file_editor.rs` is mapped to D (file editing). `task_edit_state.rs` is mapped to H; it probably also
  holds the backlog edit form, which is A behaviour, so this needs a check when H starts.
- `tasks_context.rs` (567 lines) is a composition root that grows with each slice. Slice A ports its persistence
  and use-case wiring into `infrastructure/di/tasks_module.go`; the rest is wired per slice.
- Rust tests: `task_repository_adapter_test.rs` is A; the others follow the slice of their adapter.
- No Rust unit test in `src/modules/tasks` was reassigned from its file; tests are ported with their file.

## Go status of slice F (F5 use cases)

| Rust file | Go file | Tests (Rust parity + rule tests) |
|---|---|---|
| `usecases/start_task.rs` | `application/start_task.go` | 17 + 5 |
| `usecases/send_message.rs` | `application/send_message.go` | 9 + 3 |
| `usecases/set_agent_mode.rs` | `application/set_agent_mode.go` | 3 + 1 |
| `usecases/abort_task.rs` | `application/abort_task.go` | 2 + 1 |
| `usecases/reconcile_tasks.rs` | `application/reconcile_tasks.go` | 4 + 2 |

- `finish_task.rs` is not in F: the table above puts `finish`, with reset and discard, in slice E. It is ported there (see Go status of slice E).
- Rule tests are the Go-only or Rust-untested rules; each is marked `Rule from source:` in its test file.
- `MessageMode` lives in `domain/models/agent_command.go` (Rust `agent_command.rs`). Its zero value is not a mode.
- The Rust `resume_task` continuation is still a direct `runtime.Send` in `resume_rate_limited_tasks.go` (L26): switching it to `SendMessage` changes the constructor, which the DI module calls, so it is left for the wiring step.

## Go status of slice E (finish, reset, discard)

| Rust file | Go file | Tests (Rust parity + rule tests) |
|---|---|---|
| `usecases/finish_task.rs` | `application/finish_task.go` | 4 + 6 |
| `usecases/reset_task.rs` | `application/reset_task.go` | 5 + 5 |
| `usecases/discard_task.rs` | `application/discard_task.go` | 2 + 6 |

- Use cases of slice E: finish, reset and discard are ported, so the use-case list of E above has a Go counterpart. The diff and merge presentation is slice H.
- Ports reused, none added: `TaskLifecycle`, `outbound.Worktree` (`CommitAll`, `Remove`), `outbound.AgentRuntime` (`IsRunning`, `Stop`), `outbound.TaskRepository.ResetToBacklog` and `AttachmentCleanup` (slice D).
- Not wired: `NewFinishTask`, `NewResetTask` and `NewDiscardTask` are not in `infrastructure/di/tasks_module.go` yet; the parent wires them with the rest of the module.
- Rule tests are marked `Rule from source:` in their test files; Rust tests are marked `// Rust:`.
