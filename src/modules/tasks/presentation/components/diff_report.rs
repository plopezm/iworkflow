use leptos::portal::Portal;
use leptos::prelude::*;

use crate::core::markdown::render_markdown;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::presentation::components::diff_view::line_class;
use crate::modules::tasks::presentation::state::diff_report::{FilePatch, split_patch_by_file};

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
                        "Guardar como PDF"
                    </button>
                    <button type="button" class="btn-secondary" on:click=move |_| on_close.run(())>
                        "Cerrar"
                    </button>
                </div>
                <article class="print-report-page">
                    <h1>{title}</h1>
                    <h2>{format!("Commits ({})", diff.commits.len())}</h2>
                    <pre class="print-report-block">{diff.commits.join("\n")}</pre>
                    <h2>"Resumen"</h2>
                    <pre class="print-report-block">{diff.stat}</pre>
                    {diff
                        .truncated
                        .then(|| view! { <p>"Parche truncado: revisa el resto con git."</p> })}
                    {explanation
                        .map(|text: String| {
                            view! {
                                <h2>"Explicación del agente"</h2>
                                <div class="markdown" inner_html=render_markdown(&text)></div>
                            }
                        })}
                    <h2>"Cambios por fichero"</h2>
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
