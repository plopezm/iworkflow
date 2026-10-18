use leptos::portal::Portal;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::markdown::render_markdown;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::presentation::components::diff_view::line_class;
use crate::modules::tasks::presentation::state::diff_report::{FilePatch, split_patch_by_file};

const SAVE_AS_PDF: Msg = Msg {
    en: "Save as PDF",
    es: "Guardar como PDF",
    zh: "另存为 PDF",
    de: "Als PDF speichern",
    fr: "Enregistrer en PDF",
};

const CLOSE: Msg = Msg {
    en: "Close",
    es: "Cerrar",
    zh: "关闭",
    de: "Schließen",
    fr: "Fermer",
};

const COMMITS: Msg = Msg {
    en: "Commits ({n})",
    es: "Commits ({n})",
    zh: "提交（{n}）",
    de: "Commits ({n})",
    fr: "Commits ({n})",
};

const SUMMARY: Msg = Msg {
    en: "Summary",
    es: "Resumen",
    zh: "摘要",
    de: "Zusammenfassung",
    fr: "Résumé",
};

const TRUNCATED: Msg = Msg {
    en: "Patch truncated: review the rest with git.",
    es: "Parche truncado: revisa el resto con git.",
    zh: "补丁已截断：请使用 git 查看其余部分。",
    de: "Patch gekürzt: Den Rest mit git prüfen.",
    fr: "Patch tronqué : consultez le reste avec git.",
};

const AGENT_EXPLANATION: Msg = Msg {
    en: "Agent explanation",
    es: "Explicación del agente",
    zh: "智能体说明",
    de: "Erklärung des Agents",
    fr: "Explication de l’agent",
};

const CHANGES_BY_FILE: Msg = Msg {
    en: "Changes by file",
    es: "Cambios por fichero",
    zh: "按文件划分的更改",
    de: "Änderungen pro Datei",
    fr: "Modifications par fichier",
};

/// Opens the browser print dialog (where "Save as PDF" is available).
fn print_page() {
    #[cfg(feature = "hydrate")]
    if let Some(window) = web_sys::window() {
        let _ = window.print();
    }
}

/// Printable report overlay: title, commits, summary, optional agent explanation and the
/// patch grouped per file. Rendered into `<body>` so ancestors never clip the printout.
#[component]
pub fn DiffReport(
    #[prop(into)] title: String,
    diff: TaskDiffDto,
    #[prop(optional, into)] explanation: Option<String>,
    on_close: Callback<()>,
) -> impl IntoView {
    view! {
        <Portal>
            {report_body(title.clone(), diff.clone(), explanation.clone(), on_close)}
        </Portal>
    }
}

fn report_body(
    title: String,
    diff: TaskDiffDto,
    explanation: Option<String>,
    on_close: Callback<()>,
) -> impl IntoView {
    let files: Vec<FilePatch> = split_patch_by_file(&diff.patch);
    view! {
            <div class="print-report">
                <div class="print-report-toolbar">
                    <button type="button" class="btn-primary" on:click=move |_| print_page()>
                        {SAVE_AS_PDF.get()}
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        {CLOSE.get()}
                    </button>
                </div>
                <article class="print-report-page">
                    <h1>{title}</h1>
                    <h2>{COMMITS.fmt(&[("n", &diff.commits.len().to_string())])}</h2>
                    <pre class="print-report-block">{diff.commits.join("\n")}</pre>
                    <h2>{SUMMARY.get()}</h2>
                    <pre class="print-report-block">{diff.stat}</pre>
                    {diff
                        .truncated
                        .then(|| view! { <p>{TRUNCATED.get()}</p> })}
                    {explanation
                        .map(|text: String| {
                            view! {
                                <h2>{AGENT_EXPLANATION.get()}</h2>
                                <div class="markdown" inner_html=render_markdown(&text)></div>
                            }
                        })}
                    <h2>{CHANGES_BY_FILE.get()}</h2>
                    {files
                        .into_iter()
                        .map(|file: FilePatch| {
                            view! {
                                <section class="print-report-file">
                                    <pre class="diff">
                                        {file
                                            .lines
                                            .into_iter()
                                            .map(|line: String| {
                                                view! { <div class=line_class(&line)>{line.clone()}</div> }
                                            })
                                            .collect_view()}
                                    </pre>
                                </section>
                            }
                        })
                        .collect_view()}
                </article>
            </div>
    }
}
