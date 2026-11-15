# Documentation GIFs

The GIFs in this folder are recordings of the real application, produced by a script so they can be regenerated
when the UI changes.

| File | Shows |
| --- | --- |
| `task-graph.gif` | The workflow graph: three tasks chained on the board by dragging from a node's output port. The first task runs, is finished, reviewed and merged, and the successors start on their own. |
| `task-review.gif` | A task started from the board, followed live (reasoning, tool calls, reply), finished with "Finish and review", its diff read in a tab, then merged. |
| `usage-limits.gif` | The usage bar in the sidebar footer: Claude at the normal level, Codex at warn and critical levels, zoomed for reading. |
| `task-hooks.gif` | The Hooks screen: a script hook is created, assigned to a project before a task starts, tested, and its run is shown in the task. |

## Regenerate

```bash
scripts/docs/record_gifs.sh                              # all scenes -> docs/media/*.gif
DOCS_GIF_SCENES=task-hooks scripts/docs/record_gifs.sh   # one scene
```

The script builds the server, runs `scripts/docs/record_gifs.cjs` through `end2end/go/run_go.sh`, and converts
each recording to a GIF with ffmpeg (8 fps, 900 px wide, two-pass palette). The `.webm` recordings are kept in
`$DOCS_GIF_OUT/<run>/` (default `/tmp/iwf-docs-gifs/`), so a failed scene can be inspected: it is kept as
`<scene>.failed.webm` and is not converted.

## Requirements

- Go (the server is built with `-tags sherpa`, so the first run takes several minutes).
- Node.js and a Playwright install with its Chromium build (`PLAYWRIGHT_MODULE`, `PLAYWRIGHT_BROWSERS_PATH`,
  optionally `SMOKE_CHROMIUM_PATH`).
- Python 3 (the run helpers and the demo agent).
- ffmpeg.
- The Tailwind CSS standalone CLI, only when `web/static/app.css` is missing (`make tools css`).

## What the GIFs run against

- A throwaway data folder: `run_go.sh` creates an isolated `HOME` and `IWF_HOME` in a temporary directory, so the
  real `~/.iworkflow` is never read or written. Demo repositories are created in the same temporary directory.
- A demo agent instead of a model: `scripts/docs/demo_agent.sh` speaks the agent protocol, streams its reasoning and
  reply, makes tool calls, and really edits the task worktree (a small helper module and a README line), so the
  diff in the review is real. It uses no model and no network.
- Fake subscription logins for the usage bar (Claude 13 % / 25 %, Codex 75 % / 95 %), seeded by `run_go.sh`.
- The browser is forced to English (`iw-lang=en`).

The recordings do not show the mouse pointer natively; the recorder draws a small dot that follows it.
