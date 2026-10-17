use leptos::prelude::*;

use crate::core::components::confirm_panel::ConfirmPanel;
use crate::core::components::form::{ActionError, TextField};
use crate::core::components::layout::{Badge, ErrorText};
use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::{
    CreateMergeRequestDto, MergeRequestDto, MergeRequestStatusDto, PipelineDto, PipelineJobDto,
};
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    CreateTaskMergeRequest, MergeTaskMergeRequest,
};
use crate::modules::code_hosting::presentation::state::task_merge_request_state::TaskMergeRequestState;

const MERGE_CONFIRM: &str =
    "¿Mezclar el merge request en GitLab? La rama destino se actualizará en el servidor.";
const NO_PUSH_NOTE: &str = "La rama debe estar publicada en origin: la app no hace push.";

/// Variante de color de un estado de MR, pipeline o job de GitLab (pura).
pub fn status_variant(status: &str) -> &'static str {
    match status {
        "success" | "passed" | "merged" | "mergeable" | "can_be_merged" => "ok",
        "failed" | "cannot_be_merged" | "conflict" | "broken_status" => "error",
        "running"
        | "pending"
        | "created"
        | "preparing"
        | "waiting_for_resource"
        | "scheduled"
        | "checking"
        | "ci_still_running"
        | "opened" => "info",
        "manual" | "blocked_status" | "not_approved" | "discussions_not_resolved" => "warn",
        _ => "muted",
    }
}

/// Etiqueta en español de un estado de GitLab; los desconocidos se muestran tal cual (pura).
pub fn status_label(status: &str) -> String {
    match status {
        "success" => "correcto",
        "failed" => "fallido",
        "running" => "en curso",
        "pending" => "pendiente",
        "created" => "creado",
        "canceled" => "cancelado",
        "skipped" => "omitido",
        "manual" => "manual",
        "opened" => "abierto",
        "merged" => "mezclado",
        "closed" => "cerrado",
        "mergeable" | "can_be_merged" => "se puede mezclar",
        "cannot_be_merged" => "no se puede mezclar",
        "conflict" => "con conflictos",
        "ci_still_running" => "CI en curso",
        "ci_must_pass" => "el CI debe pasar",
        "not_approved" => "pendiente de aprobación",
        "discussions_not_resolved" => "discusiones sin resolver",
        "checking" => "comprobando",
        other => other,
    }
    .to_string()
}

#[component]
fn StatusBadge(#[prop(into)] status: String) -> impl IntoView {
    view! { <Badge text=status_label(&status) variant=status_variant(&status) /> }
}

#[component]
fn ExternalLink(#[prop(into)] href: String, children: Children) -> impl IntoView {
    view! {
        <a class="text-sm underline" href=href target="_blank" rel="noopener noreferrer">
            {children()}
        </a>
    }
}

#[component]
fn PipelineView(pipeline: PipelineDto) -> impl IntoView {
    let title: String = format!("Pipeline #{}", pipeline.id);
    view! {
        <div class="stack gap-2">
            <div class="row items-center gap-2">
                <span class="text-sm font-medium text-fg">{title}</span>
                <StatusBadge status=pipeline.status />
                <ExternalLink href=pipeline.web_url>"Abrir en GitLab"</ExternalLink>
            </div>
            <ul class="stack gap-1">
                {pipeline
                    .jobs
                    .into_iter()
                    .map(|job: PipelineJobDto| {
                        view! {
                            <li class="row items-center gap-2 text-sm">
                                <StatusBadge status=job.status />
                                <span class="text-fg">{job.name}</span>
                                <span class="muted">{format!("({})", job.stage)}</span>
                                <ExternalLink href=job.web_url>"log"</ExternalLink>
                            </li>
                        }
                    })
                    .collect_view()}
            </ul>
        </div>
    }
}

#[component]
fn MergeRequestView(state: TaskMergeRequestState, status: MergeRequestStatusDto) -> impl IntoView {
    let merge_request: MergeRequestDto = status.merge_request;
    let open: bool = merge_request.is_open();
    let busy: Signal<bool> = state.busy();
    let confirming: RwSignal<bool> = RwSignal::new(false);
    let heading: String = format!("!{} {}", merge_request.iid, merge_request.title);
    let branches: String = format!(
        "{} → {} · {}",
        merge_request.source_branch, merge_request.target_branch, status.project_path
    );
    let on_confirm: Callback<()> = Callback::new(move |_| {
        confirming.set(false);
        state.merge.dispatch(MergeTaskMergeRequest {
            task_id: state.task_id.get_value(),
        });
    });
    let on_cancel: Callback<()> = Callback::new(move |_| confirming.set(false));
    view! {
        <div class="stack gap-3">
            <div class="stack gap-1">
                <span class="text-sm font-medium text-fg">{heading}</span>
                <span class="muted text-xs">{branches}</span>
                <div class="row flex-wrap items-center gap-2">
                    <StatusBadge status=merge_request.state.clone() />
                    {open
                        .then(|| {
                            view! { <StatusBadge status=merge_request.merge_status.clone() /> }
                        })}
                    {merge_request
                        .has_conflicts
                        .then(|| view! { <Badge text="con conflictos" variant="error" /> })}
                    <ExternalLink href=merge_request.web_url>"Abrir MR en GitLab"</ExternalLink>
                </div>
            </div>
            {match status.pipeline {
                Some(pipeline) => view! { <PipelineView pipeline /> }.into_any(),
                None => view! { <p class="muted text-sm">"Sin pipeline de CI."</p> }.into_any(),
            }}
            {open
                .then(|| {
                    view! {
                        <Show
                            when=move || confirming.get()
                            fallback=move || {
                                view! {
                                    <button
                                        type="button"
                                        class="btn-primary"
                                        disabled=move || busy.get()
                                        on:click=move |_| confirming.set(true)
                                    >
                                        "Mezclar MR"
                                    </button>
                                }
                            }
                        >
                            <ConfirmPanel
                                message=MERGE_CONFIRM
                                confirm_label="Mezclar"
                                pending=busy
                                on_confirm
                                on_cancel
                            />
                        </Show>
                    }
                })}
            <ActionError result=state.merge.value().into() />
        </div>
    }
}

#[component]
fn CreateMergeRequestView(state: TaskMergeRequestState) -> impl IntoView {
    let busy: Signal<bool> = state.busy();
    let on_create = move |_| {
        let title: String = state.title.get_untracked();
        state.create.dispatch(CreateTaskMergeRequest {
            input: CreateMergeRequestDto {
                task_id: state.task_id.get_value(),
                title: Some(title).filter(|t: &String| !t.trim().is_empty()),
                description: None,
            },
        });
    };
    view! {
        <div class="stack gap-3">
            <p class="muted text-sm">"Esta rama aún no tiene merge request."</p>
            <TextField
                label="Título (opcional, por defecto el de la tarea)"
                value=state.title
            />
            <p class="muted text-xs">{NO_PUSH_NOTE}</p>
            <div>
                <button
                    type="button"
                    class="btn-primary"
                    disabled=move || busy.get()
                    on:click=on_create
                >
                    "Crear merge request"
                </button>
            </div>
            <ActionError result=state.create.value().into() />
        </div>
    }
}

/// Panel de GitLab de una tarea: crear MR, ver su estado y CI, y mezclar.
#[component]
pub fn TaskMergeRequestPanel(#[prop(into)] task_id: String) -> impl IntoView {
    let state: TaskMergeRequestState = TaskMergeRequestState::new(task_id);
    view! {
        <div class="stack gap-3">
            <div class="row items-center justify-between">
                <span class="action-section">"GitLab"</span>
                <button
                    type="button"
                    class="btn-secondary min-h-8 px-3 py-1 text-xs"
                    on:click=move |_| state.refresh()
                >
                    "Actualizar"
                </button>
            </div>
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                {move || Suspend::new(async move {
                    match state.status.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(Some(status)) => {
                            view! { <MergeRequestView state status /> }.into_any()
                        }
                        Ok(None) => view! { <CreateMergeRequestView state /> }.into_any(),
                    }
                })}
            </Transition>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ci_statuses_map_to_semantic_variants() {
        assert_eq!(status_variant("success"), "ok");
        assert_eq!(status_variant("failed"), "error");
        assert_eq!(status_variant("running"), "info");
        assert_eq!(status_variant("manual"), "warn");
        assert_eq!(status_variant("canceled"), "muted");
        assert_eq!(status_variant("algo-nuevo"), "muted");
    }

    #[test]
    fn labels_are_spanish_and_unknown_statuses_pass_through() {
        assert_eq!(status_label("failed"), "fallido");
        assert_eq!(status_label("merged"), "mezclado");
        assert_eq!(status_label("algo-nuevo"), "algo-nuevo");
    }
}
