use leptos::prelude::*;
use leptos_router::hooks::use_params_map;

use crate::core::components::layout::{Card, ErrorText};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    MergeOutcomeDto, TaskDto, TaskStatusDto,
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

#[component]
pub fn TaskDetailPage() -> impl IntoView {
    let params = use_params_map();
    let task_id: String = params.with_untracked(|p| p.get("id").unwrap_or_default());
    let state: TaskDetailState = TaskDetailState::new(task_id);

    let handlers: TaskActionHandlers = TaskActionHandlers {
        abort: Callback::new(move |_| state.abort()),
        finish: Callback::new(move |_| state.finish()),
        resume: Callback::new(move |_| state.resume()),
        merge: Callback::new(move |_| state.merge()),
        discard: Callback::new(move |_| state.discard()),
        toggle_diff: Callback::new(move |_| state.show_diff.update(|v: &mut bool| *v = !*v)),
    };
    let busy: Signal<bool> = Signal::derive(move || {
        state.finish.pending().get()
            || state.merge.pending().get()
            || state.discard.pending().get()
            || state.resume.pending().get()
    });
    let pending_dialog: Memo<Option<DialogRequestDto>> =
        Memo::new(move |_| state.transcript.with(Transcript::pending_dialog));
    let on_answer: Callback<(String, DialogAnswerDto)> =
        Callback::new(move |(dialog_id, answer)| state.answer_dialog(dialog_id, answer));
    let on_send: Callback<()> = Callback::new(move |_| state.send_message());

    view! {
        <Transition fallback=|| view! { <p class="muted">"Cargando tarea…"</p> }>
            {move || Suspend::new(async move {
                match state.task.await {
                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    Ok(task) => {
                        let task: TaskDto = task;
                        let agent_busy: bool = matches!(
                            task.status,
                            TaskStatusDto::Running | TaskStatusDto::Starting
                        );
                        let can_message: bool = task.actions.can_message;
                        view! {
                            <section class="card">
                                <div class="row">
                                    <h1 class="task-title">{task.title.clone()}</h1>
                                    <TaskStatusBadge status=task.status />
                                </div>
                                <p class="muted">
                                    "Rama " <code>{task.branch.clone()}</code> " desde "
                                    <code>{task.base_branch.clone()}</code> " · worktree "
                                    <code>{task.worktree_path.clone()}</code>
                                </p>
                                {task.error.clone().map(|e: String| view! { <ErrorText message=e /> })}
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
                                                view! { <p class="notice notice-info">"Integrada en la rama base."</p> }
                                                    .into_any()
                                            }
                                            MergeOutcomeDto::Conflict { files } => {
                                                view! {
                                                    <p class="notice notice-warn">
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
                            </section>
                            <details class="card prompt">
                                <summary>"Prompt inicial"</summary>
                                <pre>{task.prompt.clone()}</pre>
                            </details>
                            <Show when=move || state.show_diff.get()>
                                <Card title="Cambios">
                                    <Suspense fallback=|| view! { <p class="muted">"Calculando diff…"</p> }>
                                        {move || Suspend::new(async move {
                                            match state.diff.await {
                                                Some(Ok(diff)) => view! { <DiffView diff /> }.into_any(),
                                                Some(Err(e)) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                                None => ().into_any(),
                                            }
                                        })}
                                    </Suspense>
                                </Card>
                            </Show>
                            <div class="task-layout">
                                <div class="stack">
                                    <TranscriptView transcript=state.transcript />
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
                                    {can_message
                                        .then(|| {
                                            view! {
                                                <Composer
                                                    message=state.message
                                                    mode=state.mode
                                                    agent_busy=Signal::derive(move || agent_busy)
                                                    pending=state.send.pending()
                                                    on_send
                                                />
                                            }
                                        })}
                                </div>
                                <AgentSidebar transcript=state.transcript connected=state.connected />
                            </div>
                        }
                            .into_any()
                    }
                }
            })}
        </Transition>
    }
}
