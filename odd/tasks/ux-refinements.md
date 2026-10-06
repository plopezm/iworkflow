# ux-refinements

Goal: UX pass over the mission-control redesign, found by reviewing the app with seeded data
(board with 5 tasks, task detail with a pending dialog). Keep Spanish UI copy.

- [x] 1. TaskCard: one attention indicator (no duplicate badge), title on up to 2 lines, prompt shown only when it adds info, stage-colored left rail on every card, quiet secondary actions (Editar/Borrar ghost, Borrar turns red on hover)
- [x] 2. Board lanes: attention-first stable ordering inside a lane (unit test) + labelled counters ("3 tareas" / "1 atención")
- [x] 3. Task detail: shortened branch in the header (full value in `title`), launch/args notice collapsed in a `<details>`, worktree path as small copy-friendly block
- [x] 4. Empty states with a call to action (sidebar workspaces, board)
- [x] 5. fmt, clippy ssr/hydrate, tests, screenshots check; commit
