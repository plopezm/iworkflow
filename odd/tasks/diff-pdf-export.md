# diff-pdf-export

Goal: above the Diff tab results (task detail right panel) add two buttons: "Exportar PDF" (plain diff report)
and "PDF con explicaciones" (the running agent is asked to explain the changes and decisions; its answer is
embedded in the report). PDF is produced by the browser print dialog ("Save as PDF") from a printable report
overlay (decisions by user: explanations from the agent, browser printing).

- [x] 1. Pure logic + tests: explanation prompt, patch file splitting, transcript lookup of the agent's explanation
- [x] 2. Report state (`diff_report_state.rs`): idle / plain / waiting-for-agent / ready, sends prompt via existing send action
- [x] 3. `DiffReport` printable component (Portal overlay) + `@media print` CSS
- [x] 4. Buttons above the diff in `DiffView` wired to the state in `task_detail_page`
- [x] 5. fmt, clippy ssr/hydrate, tests; commit
