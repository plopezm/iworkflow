use leptos::prelude::*;
use leptos_router::NavigateOptions;
use leptos_router::components::A;
use leptos_router::hooks::{use_navigate, use_params_map};

use crate::core::browser::confirm;
use crate::core::components::form::{ActionError, TextArea, TextField};
use crate::core::components::layout::ErrorText;
use crate::core::components::modal::Modal;
use crate::core::components::shell::{CollapsePanel, PanelSide, TabBar, TabItem};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    MergeOutcomeDto, TaskDto, TaskStageDto, TaskStatusDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
    DialogAnswerDto, DialogRequestDto,
};
use crate::modules::tasks::presentation::components::agent_sidebar::AgentSidebar;
use crate::modules::tasks::presentation::components::composer::Composer;
use crate::modules::tasks::presentation::components::dialog_panel::DialogPanel;
use crate::modules::tasks::presentation::components::diff_view::DiffView;
use crate::modules::tasks::presentation::components::task_actions::{
    TaskActionHandlers, TaskActions,
};
use crate::modules::tasks::presentation::components::task_status_badge::TaskStatusBadge;
use crate::modules::tasks::presentation::components::transcript_view::TranscriptView;
use crate::modules::tasks::presentation::state::task_detail_state::TaskDetailState;
use crate::modules::tasks::presentation::state::transcript::Transcript;

const TAB_DIFF: &str = "diff";
const TAB_ACTIONS: &str = "acciones";
const TAB_STATE: &str = "estado";

/// Tarjeta simplificada de una tarea en Backlog: prompt y acciones de iniciar/editar/borrar.
#[component]
fn BacklogView(task: TaskDto, state: TaskDetailState) -> impl IntoView {
    let navigate = use_navigate();
    let task_for_edit: TaskDto = task.clone();
    let busy: Signal<bool> =
        Signal::derive(move || state.start.pending().get() || state.delete.pending().get());

    Effect::new(move |_| {
        let deleted: bool = state
            .delete
            .value()
            .with(|r: &Option<Result<(), ServerFnError>>| matches!(r, Some(Ok(()))));
        if deleted {
            navigate("/", NavigateOptions::default());
        }
    });
    Effect::new(move |_| {
        let updated: bool = state
            .update
            .value()
            .with(|r: &Option<Result<TaskDto, ServerFnError>>| matches!(r, Some(Ok(_))));
        if updated {
            state.close_edit();
        }
    });

    view! {
        <section class="card stack animate-slide-up">
            <div class="row justify-end">
                <TaskStatusBadge status=task.status />
            </div>
            <p class="whitespace-pre-wrap text-fg">{task.prompt.clone()}</p>
            {move || state.action_error().map(|e: String| view! { <ErrorText message=e /> })}
            <div class="row">
                {task
                    .actions
                    .can_start
                    .then(|| {
                        view! {
                            <button
                                class="btn-primary"
                                disabled=move || busy.get()
                                on:click=move |_| state.start()
                            >
                                "Iniciar"
                            </button>
                        }
                    })}
                {task
                    .actions
                    .can_edit
                    .then(|| {
                        let task: TaskDto = task_for_edit.clone();
                        view! {
                            <button class="btn-secondary" on:click=move |_| state.open_edit(&task)>
                                "Editar"
                            </button>
                        }
                    })}
                {task
                    .actions
                    .can_delete
                    .then(|| {
                        view! {
                            <button
                                class="btn-danger"
                                disabled=move || busy.get()
                                on:click=move |_| {
                                    if confirm("¿Borrar esta tarea de la lista de pendientes?") {
                                        state.delete();
                                    }
                                }
                            >
                                "Borrar"
                            </button>
                        }
                    })}
            </div>
        </section>
        <Modal
            title=String::from("Editar tarea")
            open=state.edit_open
            on_close=Callback::new(move |_| state.close_edit())
        >
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    state.submit_edit();
                }
            >
                <TextField
                    label="Título (opcional)"
                    value=state.edit_title
                    placeholder="Se deriva del prompt"
                />
                <TextArea
                    label="Prompt"
                    value=state.edit_prompt
                    placeholder="Describe lo que debe hacer el agente…"
                    required=true
                />
                <ActionError result=state.update.value().into() />
                <div class="row justify-end gap-2">
                    <button type="button" class="btn-secondary" on:click=move |_| state.close_edit()>
                        "Cancelar"
                    </button>
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || state.update.pending().get()
                    >
                        {move || if state.update.pending().get() { "Guardando…" } else { "Guardar cambios" }}
                    </button>
                </div>
            </form>
        </Modal>
    }
}

/// Página de detalle. El router reutiliza este componente al pasar entre dos `/tasks/:id`
/// (solo cambia el signal de parámetros), así que el contenido se reconstruye por cada id.
#[component]
pub fn TaskDetailPage() -> impl IntoView {
    let params = use_params_map();
    let task_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <TaskDetailContent task_id=task_id.get() /> }} }
}

#[component]
fn TaskDetailContent(task_id: String) -> impl IntoView {
    let state: TaskDetailState = TaskDetailState::new(task_id);
    let right_open: RwSignal<bool> = RwSignal::new(true);
    let right_overlay: RwSignal<bool> = RwSignal::new(false);
    let active_tab: RwSignal<String> = RwSignal::new(TAB_ACTIONS.to_string());
    let on_select_tab: Callback<String> = Callback::new(move |id: String| {
        state.show_diff.set(id == TAB_DIFF);
        active_tab.set(id);
    });

    let handlers: TaskActionHandlers = TaskActionHandlers {
        abort: Callback::new(move |_| state.abort()),
        finish: Callback::new(move |_| state.finish()),
        resume: Callback::new(move |_| state.resume()),
        merge: Callback::new(move |_| state.merge()),
        discard: Callback::new(move |_| state.discard()),
        toggle_diff: Callback::new(move |_| {
            let next: &str = if state.show_diff.get_untracked() {
                TAB_ACTIONS
            } else {
                TAB_DIFF
            };
            on_select_tab.run(next.to_string());
        }),
    };
    let busy: Signal<bool> = Signal::derive(move || {
        state.finish.pending().get()
            || state.merge.pending().get()
            || state.discard.pending().get()
            || state.resume.pending().get()
            || state.abort.pending().get()
    });
    let pending_dialog: Memo<Option<DialogRequestDto>> =
        Memo::new(move |_| state.transcript.with(Transcript::pending_dialog));
    let on_answer: Callback<(String, DialogAnswerDto)> =
        Callback::new(move |(dialog_id, answer)| state.answer_dialog(dialog_id, answer));
    let on_send: Callback<()> = Callback::new(move |_| state.send_message());

    let loaded_task: RwSignal<Option<TaskDto>> = RwSignal::new(None);
    Effect::new(move |_| {
        if let Some(Ok(task)) = state.task.get() {
            loaded_task.set(Some(task));
        }
    });

    let shows_workspace: Memo<bool> = Memo::new(move |_| {
        loaded_task
            .with(|t: &Option<TaskDto>| matches!(t, Some(t) if t.stage != TaskStageDto::Backlog))
    });
    let can_message: Memo<bool> = Memo::new(move |_| {
        loaded_task.with(|t: &Option<TaskDto>| matches!(t, Some(t) if t.actions.can_message))
    });
    let agent_busy: Memo<bool> = Memo::new(move |_| {
        loaded_task.with(|t: &Option<TaskDto>| {
            matches!(
                t,
                Some(t) if matches!(t.status, TaskStatusDto::Running | TaskStatusDto::Starting)
            )
        })
    });
    let prompt: Memo<String> = Memo::new(move |_| {
        loaded_task.with(|t: &Option<TaskDto>| {
            t.as_ref()
                .map(|t: &TaskDto| t.prompt.clone())
                .unwrap_or_default()
        })
    });

    view! {
        <Show
            when=move || !state.removed.get()
            fallback=|| {
                view! {
                    <section class="card stack m-4">
                        <h1 class="task-title">"Tarea eliminada"</h1>
                        <p class="muted">"Esta tarea ya no existe."</p>
                        <A href="/" attr:class="btn-secondary">
                            "Volver al tablero"
                        </A>
                    </section>
                }
            }
        >
            <div class="flex h-full min-h-0">
                <div class="flex min-w-0 flex-1 flex-col">
                    <div class="flex shrink-0 items-center gap-2 border-b border-border bg-surface px-4 py-2.5">
                        <div class="min-w-0 flex-1">
                            <Transition fallback=|| view! { <p class="muted">"Cargando tarea…"</p> }>
                                {move || Suspend::new(async move {
                                    match state.task.await {
                                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                        Ok(task) => {
                                            let task: TaskDto = task;
                                            let is_backlog: bool = task.stage == TaskStageDto::Backlog;
                                            view! {
                                                <h1 class="truncate text-sm font-bold">{task.title.clone()}</h1>
                                                {(!is_backlog)
                                                    .then(|| {
                                                        view! {
                                                            <p class="truncate text-[11px] text-muted">
                                                                "Rama "
                                                                <code class="font-mono">{task.branch.clone()}</code>
                                                                " desde "
                                                                <code class="font-mono">{task.base_branch.clone()}</code>
                                                            </p>
                                                        }
                                                    })}
                                                {task.error.clone().map(|e: String| view! { <ErrorText message=e /> })}
                                            }
                                                .into_any()
                                        }
                                    }
                                })}
                            </Transition>
                        </div>
                        <Show when=move || shows_workspace.get()>
                            <button
                                class="btn-secondary min-h-9 px-3 py-1 text-xs md:hidden"
                                on:click=move |_| right_overlay.set(true)
                            >
                                "Panel"
                            </button>
                        </Show>
                    </div>
                    <Transition fallback=|| ()>
                        {move || Suspend::new(async move {
                            match state.task.await {
                                Ok(task) if task.stage == TaskStageDto::Backlog => {
                                    view! {
                                        <div class="min-h-0 flex-1 overflow-y-auto p-4">
                                            <BacklogView task state />
                                        </div>
                                    }
                                        .into_any()
                                }
                                _ => ().into_any(),
                            }
                        })}
                    </Transition>
                    <Show when=move || shows_workspace.get()>
                        <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                            <TranscriptView transcript=state.transcript />
                        </div>
                        <div class="stack max-h-[60vh] shrink-0 gap-2 overflow-y-auto border-t border-border bg-surface p-3">
                            {move || {
                                pending_dialog
                                    .get()
                                    .map(|request: DialogRequestDto| {
                                        let dialog_id: String = request.id.clone();
                                        view! {
                                            <DialogPanel
                                                request
                                                on_answer=Callback::new(move |answer: DialogAnswerDto| {
                                                    on_answer.run((dialog_id.clone(), answer))
                                                })
                                            />
                                        }
                                    })
                            }}
                            <div class="flex items-center gap-2 rounded-full border border-border bg-bg px-3 py-1 text-xs text-muted">
                                {move || {
                                    loaded_task
                                        .get()
                                        .map(|t: TaskDto| view! { <TaskStatusBadge status=t.status /> })
                                }}
                                <span>
                                    {move || if state.connected.get() { "en vivo" } else { "desconectado" }}
                                </span>
                            </div>
                            <Show when=move || can_message.get()>
                                <Composer
                                    message=state.message
                                    mode=state.mode
                                    agent_busy=agent_busy
                                    pending=state.send.pending()
                                    on_send
                                />
                            </Show>
                        </div>
                    </Show>
                </div>
                <Show when=move || shows_workspace.get()>
                    <CollapsePanel
                        side=PanelSide::Right
                        open=right_open
                        overlay_open=right_overlay
                        width="md:w-[26rem]"
                        label="Panel de la tarea"
                    >
                        <div class="flex shrink-0 items-center justify-between border-b border-border px-3 py-2 md:hidden">
                            <span class="text-xs font-bold tracking-widest uppercase">"Tarea"</span>
                            <button
                                class="btn-ghost min-h-8 px-2 py-1 text-xs"
                                on:click=move |_| right_overlay.set(false)
                            >
                                "Cerrar"
                            </button>
                        </div>
                        <TabBar
                            tabs=vec![
                                TabItem { id: TAB_DIFF, label: "Diff" },
                                TabItem { id: TAB_ACTIONS, label: "Acciones" },
                                TabItem { id: TAB_STATE, label: "Estado" },
                            ]
                            active=active_tab
                            on_select=on_select_tab
                        />
                        <div class="stack min-h-0 flex-1 overflow-y-auto p-3">
                            <Show when=move || active_tab.get() == TAB_DIFF>
                                <Suspense fallback=|| view! { <p class="muted">"Calculando diff…"</p> }>
                                    {move || Suspend::new(async move {
                                        match state.diff.await {
                                            Some(Ok(diff)) => view! { <DiffView diff /> }.into_any(),
                                            Some(Err(e)) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                            None => {
                                                view! { <p class="muted">"Sin diff disponible para esta tarea."</p> }
                                                    .into_any()
                                            }
                                        }
                                    })}
                                </Suspense>
                            </Show>
                            <Show when=move || active_tab.get() == TAB_ACTIONS>
                                <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                                    {move || Suspend::new(async move {
                                        match state.task.await {
                                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                            Ok(task) => {
                                                let task: TaskDto = task;
                                                view! {
                                                    <TaskActions
                                                        actions=task.actions
                                                        handlers
                                                        diff_visible=state.show_diff
                                                        busy
                                                    />
                                                    {move || state.action_error().map(|e: String| view! { <ErrorText message=e /> })}
                                                    {move || {
                                                        state
                                                            .merge
                                                            .value()
                                                            .get()
                                                            .and_then(|r| r.ok())
                                                            .map(|outcome: MergeOutcomeDto| match outcome {
                                                                MergeOutcomeDto::Merged => {
                                                                    view! { <p class="notice notice-info animate-fade-in">"Integrada en la rama base."</p> }
                                                                        .into_any()
                                                                }
                                                                MergeOutcomeDto::Conflict { files } => {
                                                                    view! {
                                                                        <p class="notice notice-warn animate-fade-in">
                                                                            {format!(
                                                                                "Conflictos en {}. Reanuda el agente para resolverlos o hazlo a mano en el worktree.",
                                                                                files.join(", "),
                                                                            )}
                                                                        </p>
                                                                    }
                                                                        .into_any()
                                                                }
                                                            })
                                                    }}
                                                    <p class="muted break-all">
                                                        "Worktree "
                                                        <code class="font-mono text-xs">{task.worktree_path.clone()}</code>
                                                    </p>
                                                }
                                                    .into_any()
                                            }
                                        }
                                    })}
                                </Transition>
                                <details class="card prompt">
                                    <summary>"Prompt inicial"</summary>
                                    <pre class="font-mono text-sm whitespace-pre-wrap">{move || prompt.get()}</pre>
                                </details>
                            </Show>
                            <Show when=move || active_tab.get() == TAB_STATE>
                                <AgentSidebar
                                    transcript=state.transcript
                                    connected=state.connected
                                    class="min-w-0"
                                />
                            </Show>
                        </div>
                    </CollapsePanel>
                </Show>
            </div>
        </Show>
    }
}
