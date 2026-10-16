use leptos::prelude::*;

use crate::core::components::confirm_panel::ConfirmPanel;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    JiraIssueDto, JiraLaneDto, JiraLaneListingDto,
};
use crate::modules::jira::presentation::components::issue_row::IssueRow;

/// «3» when nothing is filtered out, «2 de 5» when filters hide part of the lane.
pub fn lane_count_label(shown: usize, total: usize) -> String {
    if shown == total {
        total.to_string()
    } else {
        format!("{shown} de {total}")
    }
}

/// Text shown inside an expanded lane without visible issues.
pub fn empty_lane_message(total: usize) -> &'static str {
    if total == 0 {
        "Sin tareas en este carril. Pulsa «Sincronizar» para traerlas de Jira."
    } else {
        "Ninguna tarea de este carril coincide con los filtros."
    }
}

/// A collapsible group of issues fed by one JQL query. The built-in lane cannot be edited or
/// removed.
#[component]
pub fn LaneSection(
    listing: JiraLaneListingDto,
    /// `(key, ignored)`: ignore or restore an issue.
    on_ignore: Callback<(String, bool)>,
    on_toggle: Callback<(String, bool)>,
    on_edit: Callback<JiraLaneDto>,
    on_delete: Callback<String>,
    #[prop(into)] deleting: Signal<bool>,
) -> impl IntoView {
    let lane: JiraLaneDto = listing.lane;
    let collapsed: RwSignal<bool> = RwSignal::new(lane.collapsed);
    let confirming_delete: RwSignal<bool> = RwSignal::new(false);
    let count: String = lane_count_label(listing.issues.len(), listing.total);
    let empty_message: &'static str = empty_lane_message(listing.total);
    let is_empty: bool = listing.issues.is_empty();
    let toggle_id: String = lane.id.clone();
    let delete_id: String = lane.id.clone();
    let for_edit: JiraLaneDto = lane.clone();
    let is_default: bool = lane.is_default;
    let jql: String = lane.jql.clone();
    let jql_title: String = lane.jql;
    let name: String = lane.name;
    let issues: Vec<JiraIssueDto> = listing.issues;
    view! {
        <section class="overflow-hidden rounded-xl border border-border bg-surface/60">
            <header class="flex items-center gap-1 pr-2">
                <button
                    type="button"
                    class="flex min-w-0 flex-1 cursor-pointer items-center gap-3 px-3 py-2.5 text-left transition-colors duration-150 hover:bg-surface-2"
                    aria-expanded=move || (!collapsed.get()).to_string()
                    on:click=move |_| {
                        let next: bool = !collapsed.get_untracked();
                        collapsed.set(next);
                        on_toggle.run((toggle_id.clone(), next));
                    }
                >
                    <span
                        class="inline-block w-3 shrink-0 text-xs text-muted transition-transform duration-200"
                        class=("rotate-90", move || !collapsed.get())
                    >
                        "▶"
                    </span>
                    <span class="shrink-0 text-sm font-semibold text-fg">{name}</span>
                    <span class="badge badge-muted shrink-0 tabular-nums">{count}</span>
                    {is_default
                        .then(|| {
                            view! { <span class="eyebrow shrink-0">"Asignadas a mí · sin Done"</span> }
                        })}
                    {(!is_default)
                        .then(|| {
                            view! {
                                <code
                                    class="hidden min-w-0 truncate font-mono text-[11px] text-muted md:inline"
                                    title=jql_title
                                >
                                    {jql}
                                </code>
                            }
                        })}
                </button>
                {(!is_default)
                    .then(|| {
                        view! {
                            <button
                                type="button"
                                class="btn-ghost"
                                title="Editar carril"
                                on:click=move |_| on_edit.run(for_edit.clone())
                            >
                                "Editar"
                            </button>
                            <button
                                type="button"
                                class="btn-ghost"
                                title="Eliminar carril"
                                on:click=move |_| confirming_delete.set(true)
                            >
                                "Eliminar"
                            </button>
                        }
                    })}
            </header>
            <div
                class="grid transition-[grid-template-rows] duration-200 ease-out"
                class=("grid-rows-[0fr]", move || collapsed.get())
                class=("grid-rows-[1fr]", move || !collapsed.get())
            >
                <div class="min-h-0 overflow-hidden">
                    <div class="stack border-t border-border p-3">
                        {move || {
                            let delete_id: String = delete_id.clone();
                            confirming_delete
                                .get()
                                .then(|| {
                                    view! {
                                        <ConfirmPanel
                                            message="Se elimina el carril; las tareas ya convertidas o ignoradas se conservan."
                                            confirm_label="Eliminar carril"
                                            pending=deleting
                                            on_confirm=Callback::new(move |_| {
                                                on_delete.run(delete_id.clone())
                                            })
                                            on_cancel=Callback::new(move |_| confirming_delete.set(false))
                                        />
                                    }
                                })
                        }}
                        {if is_empty {
                            view! { <p class="muted">{empty_message}</p> }.into_any()
                        } else {
                            view! {
                                <ul class="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
                                    {issues
                                        .into_iter()
                                        .map(|issue: JiraIssueDto| view! { <IssueRow issue on_ignore /> })
                                        .collect_view()}
                                </ul>
                            }
                                .into_any()
                        }}
                    </div>
                </div>
            </div>
        </section>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn count_shows_the_filtered_fraction_only_when_filtering() {
        assert_eq!(lane_count_label(3, 3), "3");
        assert_eq!(lane_count_label(0, 0), "0");
        assert_eq!(lane_count_label(2, 5), "2 de 5");
    }

    #[test]
    fn empty_message_distinguishes_no_issues_from_filtered_out() {
        assert!(empty_lane_message(0).contains("Sincronizar"));
        assert!(empty_lane_message(4).contains("filtros"));
    }
}
