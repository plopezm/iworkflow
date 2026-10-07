use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::presentation::components::diff_report::DiffReport;
use crate::modules::tasks::presentation::state::diff_report_state::{DiffReportState, ReportPhase};

/// Clase CSS de una línea de un parche unificado.
pub fn line_class(line: &str) -> &'static str {
    if line.starts_with("diff --git") {
        "diff-file"
    } else if line.starts_with("+++") || line.starts_with("---") {
        "diff-meta"
    } else if line.starts_with("@@") {
        "diff-hunk"
    } else if line.starts_with('+') {
        "diff-add"
    } else if line.starts_with('-') {
        "diff-del"
    } else {
        "diff-ctx"
    }
}

#[component]
pub fn DiffView(
    diff: TaskDiffDto,
    #[prop(into)] title: String,
    report: DiffReportState,
) -> impl IntoView {
    if diff.patch.is_empty() && diff.commits.is_empty() {
        return view! { <p class="muted">"Sin cambios respecto a la rama base."</p> }.into_any();
    }
    let report_diff: TaskDiffDto = diff.clone();
    let on_close: Callback<()> = Callback::new(move |_| report.close());
    view! {
        <div class="stack">
            <div class="flex flex-wrap gap-2">
                <button type="button" class="btn-secondary" on:click=move |_| report.open_plain()>
                    "Exportar PDF"
                </button>
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| report.request_explanation()
                >
                    "PDF con explicaciones"
                </button>
            </div>
            {move || match report.phase.get() {
                ReportPhase::Idle => ().into_any(),
                ReportPhase::Waiting => {
                    view! {
                        <div class="notice animate-fade-in flex items-center justify-between gap-2">
                            <span>"Esperando la explicación del agente…"</span>
                            <button type="button" class="btn-ghost" on:click=move |_| report.close()>
                                "Cancelar"
                            </button>
                        </div>
                    }
                        .into_any()
                }
                ReportPhase::Plain => {
                    view! {
                        <DiffReport title=title.clone() diff=report_diff.clone() on_close />
                    }
                        .into_any()
                }
                ReportPhase::Ready(text) => {
                    view! {
                        <DiffReport
                            title=title.clone()
                            diff=report_diff.clone()
                            explanation=text
                            on_close
                        />
                    }
                        .into_any()
                }
            }}
            <h3>{format!("Commits ({})", diff.commits.len())}</h3>
            <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{diff.commits.join("\n")}</pre>
            <h3>"Resumen"</h3>
            <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{diff.stat}</pre>
            {diff
                .truncated
                .then(|| view! { <p class="notice notice-warn animate-fade-in">"Parche truncado: revisa el resto con git."</p> })}
            <pre class="diff">
                {diff
                    .patch
                    .lines()
                    .map(|line: &str| {
                        view! { <div class=line_class(line)>{line.to_string()}</div> }
                    })
                    .collect_view()}
            </pre>
        </div>
    }
    .into_any()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_patch_lines() {
        assert_eq!(line_class("diff --git a/x b/x"), "diff-file");
        assert_eq!(line_class("+++ b/x"), "diff-meta");
        assert_eq!(line_class("@@ -1 +1 @@"), "diff-hunk");
        assert_eq!(line_class("+nuevo"), "diff-add");
        assert_eq!(line_class("-viejo"), "diff-del");
        assert_eq!(line_class(" igual"), "diff-ctx");
    }
}
