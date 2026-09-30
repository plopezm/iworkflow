use chrono::DateTime;
use leptos::prelude::*;

use crate::core::browser::confirm;
use crate::core::components::confirm_panel::ConfirmPanel;
use crate::core::components::form::{SelectField, SelectOption};
use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::{
    AgentsFileDto, ProjectOptionDto,
};
use crate::modules::agents_files::presentation::state::agents_files_state::ApplyFeedback;

/// Fecha legible (UTC) a partir de un RFC 3339; si no se puede interpretar, el texto original.
fn format_created_at(raw: &str) -> String {
    DateTime::parse_from_rfc3339(raw)
        .map(|d: DateTime<chrono::FixedOffset>| d.format("%d/%m/%Y %H:%M UTC").to_string())
        .unwrap_or_else(|_| raw.to_string())
}

/// Un fichero AGENTS.md: datos, acción "Añadir a proyecto" y borrado.
#[component]
pub fn AgentsFileItem(
    file: AgentsFileDto,
    projects: Vec<ProjectOptionDto>,
    #[prop(into)] feedback: Signal<Option<ApplyFeedback>>,
    #[prop(into)] applying: Signal<bool>,
    on_apply: Callback<(String, String)>,
    on_confirm_overwrite: Callback<()>,
    on_dismiss: Callback<()>,
    on_delete: Callback<String>,
) -> impl IntoView {
    let project_id: RwSignal<String> = RwSignal::new(String::new());
    let has_projects: bool = !projects.is_empty();
    let options: Vec<SelectOption> = projects
        .into_iter()
        .map(|p: ProjectOptionDto| SelectOption {
            value: p.id,
            label: p.name,
        })
        .collect();
    let description: String = file.description.clone();
    let apply_id: String = file.id.clone();
    let delete_id: String = file.id.clone();
    let delete_description: String = file.description.clone();

    view! {
        <article class="card stack gap-2 animate-slide-up">
            <div class="row justify-between">
                <span class="font-medium text-fg">{description}</span>
                <button
                    type="button"
                    class="btn-danger min-h-9 rounded-lg px-2.5 py-1 text-xs"
                    on:click=move |_| {
                        if confirm(&format!("¿Eliminar \"{delete_description}\"?")) {
                            on_delete.run(delete_id.clone());
                        }
                    }
                >
                    "Eliminar"
                </button>
            </div>
            <span class="muted">{format_created_at(&file.created_at)}</span>
            <code class="break-all font-mono text-xs text-muted">{file.path}</code>
            {if has_projects {
                view! {
                    <div class="stack gap-2 sm:flex-row sm:items-end">
                        <div class="min-w-0 sm:flex-1">
                            <SelectField
                                label="Proyecto"
                                value=project_id
                                options
                                placeholder="Elige un proyecto"
                            />
                        </div>
                        <button
                            type="button"
                            class="btn-secondary"
                            disabled=move || project_id.get().is_empty() || applying.get()
                            on:click=move |_| {
                                on_apply.run((apply_id.clone(), project_id.get_untracked()));
                            }
                        >
                            "Añadir a proyecto"
                        </button>
                    </div>
                }
                    .into_any()
            } else {
                view! { <p class="muted">"No hay workspaces registrados."</p> }.into_any()
            }}
            {move || {
                feedback
                    .get()
                    .map(|f: ApplyFeedback| match f {
                        ApplyFeedback::TargetExists => {
                            view! {
                                <ConfirmPanel
                                    message=f.message()
                                    confirm_label="Sobrescribir"
                                    pending=applying
                                    on_confirm=on_confirm_overwrite
                                    on_cancel=on_dismiss
                                />
                            }
                                .into_any()
                        }
                        ApplyFeedback::Committed | ApplyFeedback::NotCommitted => {
                            let class: &'static str = if f == ApplyFeedback::Committed {
                                "text-sm text-ok"
                            } else {
                                "text-sm text-warn"
                            };
                            view! {
                                <div class="row justify-between">
                                    <p class=class>{f.message()}</p>
                                    <button
                                        type="button"
                                        class="btn-ghost"
                                        on:click=move |_| on_dismiss.run(())
                                    >
                                        "Cerrar"
                                    </button>
                                </div>
                            }
                                .into_any()
                        }
                    })
            }}
        </article>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn formats_rfc3339_as_utc_date() {
        assert_eq!(
            format_created_at("2026-01-02T03:04:05+00:00"),
            "02/01/2026 03:04 UTC"
        );
    }

    #[test]
    fn keeps_unparseable_dates_verbatim() {
        assert_eq!(format_created_at("ayer"), "ayer");
    }
}
