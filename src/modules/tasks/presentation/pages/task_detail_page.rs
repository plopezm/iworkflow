use crate::core::dock::context::{DockContext, use_dock, use_tab_nav, use_tab_params};
use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::components::busy::Loading;

use crate::core::browser::{confirm, follow_bottom, scroll_to_bottom};
use crate::core::components::layout::ErrorText;
use crate::core::components::modal::Modal;
use crate::core::components::shell::{CollapsePanel, PanelSide, TabBar, TabItem};
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::TaskAttachmentDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::subagent_task_dto::SubagentTaskDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    MergeOutcomeDto, TaskDto, TaskStageDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
    DialogAnswerDto, DialogRequestDto,
};
use crate::modules::tasks::presentation::components::composer::Composer;
use crate::modules::tasks::presentation::components::dialog_panel::DialogPanel;
use crate::modules::tasks::presentation::components::subagents_panel::SubagentsPanel;
use crate::modules::tasks::presentation::components::task_actions::{
    TaskActionHandlers, TaskActions,
};
use crate::modules::tasks::presentation::components::task_attachments_list::TaskAttachmentsList;
use crate::modules::tasks::presentation::components::task_status_badge::TaskStatusBadge;
use crate::modules::tasks::presentation::components::transcript_view::TranscriptView;
use crate::modules::tasks::presentation::state::graph_model::{
    back_to_board_path, board_return_path,
};
use crate::modules::tasks::presentation::state::task_detail_state::TaskDetailState;
use crate::modules::tasks::presentation::state::transcript::Transcript;

const ACTIONS: Msg = Msg {
    en: "Actions",
    es: "Acciones",
    zh: "操作",
    de: "Aktionen",
    fr: "Actions",
};

const START: Msg = Msg {
    en: "Start",
    es: "Iniciar",
    zh: "开始",
    de: "Starten",
    fr: "Démarrer",
};

const EDIT: Msg = Msg {
    en: "Edit",
    es: "Editar",
    zh: "编辑",
    de: "Bearbeiten",
    fr: "Modifier",
};

const CONFIRM_DELETE: Msg = Msg {
    en: "Delete this task from the to-do list?",
    es: "¿Borrar esta tarea de la lista de pendientes?",
    zh: "要从待办列表中删除此任务吗？",
    de: "Diese Aufgabe aus der To-do-Liste löschen?",
    fr: "Supprimer cette tâche de la liste des tâches à faire ?",
};

const DELETE: Msg = Msg {
    en: "Delete",
    es: "Borrar",
    zh: "删除",
    de: "Löschen",
    fr: "Supprimer",
};

const TASK_DELETED: Msg = Msg {
    en: "Task deleted",
    es: "Tarea eliminada",
    zh: "任务已删除",
    de: "Aufgabe gelöscht",
    fr: "Tâche supprimée",
};

const TASK_GONE: Msg = Msg {
    en: "This task no longer exists.",
    es: "Esta tarea ya no existe.",
    zh: "此任务已不存在。",
    de: "Diese Aufgabe existiert nicht mehr.",
    fr: "Cette tâche n'existe plus.",
};

const BACK_TO_BOARD: Msg = Msg {
    en: "Back to the board",
    es: "Volver al tablero",
    zh: "返回看板",
    de: "Zurück zum Board",
    fr: "Retour au tableau",
};

const LOADING_TASK: Msg = Msg {
    en: "Loading task…",
    es: "Cargando tarea…",
    zh: "正在加载任务…",
    de: "Aufgabe wird geladen…",
    fr: "Chargement de la tâche…",
};

const BRANCH_PREFIX: Msg = Msg {
    en: "Branch ",
    es: "Rama ",
    zh: "分支 ",
    de: "Branch ",
    fr: "Branche ",
};

const BRANCH_FROM: Msg = Msg {
    en: " from ",
    es: " desde ",
    zh: " 基于 ",
    de: " von ",
    fr: " depuis ",
};

const PANEL: Msg = Msg {
    en: "Panel",
    es: "Panel",
    zh: "面板",
    de: "Panel",
    fr: "Panneau",
};

const GO_TO_LAST_MESSAGE: Msg = Msg {
    en: "Go to the last message",
    es: "Ir al último mensaje",
    zh: "跳转到最新消息",
    de: "Zur letzten Nachricht springen",
    fr: "Aller au dernier message",
};

const LAST_MESSAGE: Msg = Msg {
    en: "↓ Last message",
    es: "↓ Último mensaje",
    zh: "↓ 最新消息",
    de: "↓ Letzte Nachricht",
    fr: "↓ Dernier message",
};

const LIVE: Msg = Msg {
    en: "live",
    es: "en vivo",
    zh: "实时",
    de: "live",
    fr: "en direct",
};

const DISCONNECTED: Msg = Msg {
    en: "disconnected",
    es: "desconectado",
    zh: "已断开连接",
    de: "getrennt",
    fr: "déconnecté",
};

const TASK_PANEL: Msg = Msg {
    en: "Task panel",
    es: "Panel de la tarea",
    zh: "任务面板",
    de: "Aufgabenpanel",
    fr: "Panneau de la tâche",
};

const TASK: Msg = Msg {
    en: "Task",
    es: "Tarea",
    zh: "任务",
    de: "Aufgabe",
    fr: "Tâche",
};

const CLOSE: Msg = Msg {
    en: "Close",
    es: "Cerrar",
    zh: "关闭",
    de: "Schließen",
    fr: "Fermer",
};

const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};

const MERGED_INTO_BASE: Msg = Msg {
    en: "Merged into the base branch.",
    es: "Integrada en la rama base.",
    zh: "已合并到基础分支。",
    de: "In den Basis-Branch integriert.",
    fr: "Intégrée dans la branche de base.",
};

const MERGE_CONFLICTS: Msg = Msg {
    en: "Conflicts in {files}. Resume the agent to resolve them or do it by hand in the worktree.",
    es: "Conflictos en {files}. Reanuda el agente para resolverlos o hazlo a mano en el worktree.",
    zh: "{files} 中存在冲突。请恢复智能体来解决，或在 worktree 中手动解决。",
    de: "Konflikte in {files}. Setze den Agent fort, um sie zu lösen, oder löse sie manuell im Worktree.",
    fr: "Conflits dans {files}. Relancez l'agent pour les résoudre ou faites-le à la main dans le worktree.",
};

const BASE_BRANCH: Msg = Msg {
    en: "Starting branch",
    es: "Rama de partida",
    zh: "起始分支",
    de: "Ausgangs-Branch",
    fr: "Branche de départ",
};

const AUTO_SCROLL: Msg = Msg {
    en: "Auto-scroll",
    es: "Scroll automático",
    zh: "自动滚动",
    de: "Automatisch scrollen",
    fr: "Défilement automatique",
};

const INITIAL_PROMPT: Msg = Msg {
    en: "Initial prompt",
    es: "Prompt inicial",
    zh: "初始提示词",
    de: "Ursprünglicher Prompt",
    fr: "Prompt initial",
};

const SUBAGENTS: Msg = Msg {
    en: "Subagents",
    es: "Subagentes",
    zh: "子智能体",
    de: "Subagents",
    fr: "Sous-agents",
};

const PANEL_WIDTH_DEFAULT: &str = "md:w-[26rem]";
const TAB_ACTIONS: &str = "acciones";

/// Pestaña adicional del panel de la tarea que aporta el composition root (p. ej. GitLab), para que
/// `tasks` no dependa de otros features.
#[derive(Clone, Copy)]
pub struct TaskExtraTab {
    pub id: &'static str,
    pub label: &'static str,
    /// Recibe el id de la tarea y devuelve el contenido de la pestaña.
    pub render: Callback<String, AnyView>,
}
/// Id del contenedor con scroll de la conversación.
const TRANSCRIPT_SCROLL_ID: &str = "task-transcript-scroll";

/// Pestañas del panel de la tarea: las propias más las adicionales, en orden (pura).
fn tab_items(extra: &[TaskExtraTab]) -> Vec<TabItem> {
    let mut tabs: Vec<TabItem> = vec![TabItem {
        id: TAB_ACTIONS,
        label: ACTIONS.get(),
    }];
    tabs.extend(extra.iter().map(|tab: &TaskExtraTab| TabItem {
        id: tab.id,
        label: tab.label,
    }));
    tabs
}

/// Route of the dock tab that shows the diff of a task (pure).
pub fn diff_path(task_id: &str) -> String {
    format!("/tasks/{task_id}/diff")
}

/// Longitud máxima (en caracteres) de la rama mostrada en la cabecera.
const BRANCH_MAX_CHARS: usize = 36;

/// Whether `value` looks like a hex id: a canonical uuid or a hex run of 8+ digits.
fn is_id_like(value: &str) -> bool {
    let hex_only: bool = value
        .chars()
        .all(|c: char| c.is_ascii_hexdigit() || c == '-');
    let digits: usize = value.chars().filter(|c: &char| *c != '-').count();
    hex_only && digits >= 8
}

/// Shortens a branch for display: drops a trailing uuid-like suffix and middle-truncates
/// what is left to `BRANCH_MAX_CHARS` characters.
pub fn short_branch(branch: &str) -> String {
    let mut base: &str = branch;
    // A uuid has 4 dashes, so try the last 1..=5 dash-separated chunks as the suffix.
    for chunks in (1..=5).rev() {
        let parts: Vec<&str> = branch.rsplitn(chunks + 1, '-').collect();
        if parts.len() == chunks + 1 {
            let suffix: String = parts[..chunks]
                .iter()
                .rev()
                .copied()
                .collect::<Vec<&str>>()
                .join("-");
            if is_id_like(&suffix) && (chunks > 1 || suffix.len() >= 8) {
                base = parts[chunks];
                break;
            }
        }
    }
    let chars: Vec<char> = base.chars().collect();
    if chars.len() <= BRANCH_MAX_CHARS {
        return base.to_string();
    }
    let keep: usize = BRANCH_MAX_CHARS - 1;
    let head: usize = keep.div_ceil(2);
    let tail: usize = keep - head;
    let start: String = chars[..head].iter().collect();
    let end: String = chars[chars.len() - tail..].iter().collect();
    format!("{start}…{end}")
}

/// Tarjeta simplificada de una tarea en Backlog: prompt y acciones de iniciar/editar/borrar.
#[component]
fn BacklogView(task: TaskDto, state: TaskDetailState) -> impl IntoView {
    let nav = use_tab_nav();
    let edit_href: String = format!("/tasks/{}/edit", task.id);
    let busy: Signal<bool> =
        Signal::derive(move || state.start.pending().get() || state.delete.pending().get());
    let board_href: String = back_to_board_path(Some(&task.workspace_id));

    Effect::new(move |_| {
        let deleted: bool = state
            .delete
            .value()
            .with(|r: &Option<Result<(), ServerFnError>>| matches!(r, Some(Ok(()))));
        if deleted {
            nav.close_and_open(&board_href);
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
                                class:is-busy=move || busy.get()
                                on:click=move |_| state.start()
                            >
                                {START.get()}
                            </button>
                        }
                    })}
                {task
                    .actions
                    .can_edit
                    .then(|| {
                        view! {
                            <TabLink href=edit_href class="btn-secondary">
                                {EDIT.get()}
                            </TabLink>
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
                                class:is-busy=move || busy.get()
                                on:click=move |_| {
                                    if confirm(CONFIRM_DELETE.get()) {
                                        state.delete();
                                    }
                                }
                            >
                                {DELETE.get()}
                            </button>
                        }
                    })}
            </div>
        </section>
    }
}

/// Página de detalle. El router reutiliza este componente al pasar entre dos `/tasks/:id`
/// (solo cambia el signal de parámetros), así que el contenido se reconstruye por cada id.
#[component]
pub fn TaskDetailPage(#[prop(optional)] extra_tabs: Vec<TaskExtraTab>) -> impl IntoView {
    let params = use_tab_params();
    let task_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <TaskDetailContent task_id=task_id.get() extra_tabs=extra_tabs.clone() /> }} }
}

#[component]
fn TaskDetailContent(task_id: String, extra_tabs: Vec<TaskExtraTab>) -> impl IntoView {
    let state: TaskDetailState = TaskDetailState::new(task_id);
    // Several task tabs can be mounted at once, so the scroll container id is per task.
    let scroll_id: StoredValue<String> = StoredValue::new(format!(
        "{TRANSCRIPT_SCROLL_ID}-{}",
        state.task_id.get_value()
    ));
    let nav = use_tab_nav();
    // Restablecer deja la tarea en Backlog, que no tiene pantalla propia: vuelta al tablero.
    Effect::new(move |_| {
        let reset_done: bool = state
            .reset
            .value()
            .with(|r: &Option<Result<(), ServerFnError>>| matches!(r, Some(Ok(()))));
        if reset_done {
            nav.close_and_open(&board_return_path(state.task.get_untracked()));
        }
    });
    let extra_tabs: StoredValue<Vec<TaskExtraTab>> = StoredValue::new(extra_tabs);
    let right_open: RwSignal<bool> = RwSignal::new(true);
    let right_overlay: RwSignal<bool> = RwSignal::new(false);
    let active_tab: RwSignal<String> = RwSignal::new(TAB_ACTIONS.to_string());
    let panel_width: Signal<String> = Signal::derive(|| PANEL_WIDTH_DEFAULT.to_string());
    let on_select_tab: Callback<String> = Callback::new(move |id: String| active_tab.set(id));
    let diff_path: String = diff_path(&state.task_id.get_value());

    let handlers: TaskActionHandlers = TaskActionHandlers {
        abort: Callback::new(move |_| state.abort()),
        finish: Callback::new(move |_| state.finish()),
        resume: Callback::new(move |_| state.resume()),
        merge: Callback::new(move |_| state.merge()),
        discard: Callback::new(move |_| state.discard()),
        reset: Callback::new(move |_| state.reset()),
        open_diff: Callback::new(move |_| nav.open(&diff_path)),
        show_subagents: Callback::new(move |_| state.show_subagents.set(true)),
    };
    let busy: Signal<bool> = Signal::derive(move || {
        state.finish.pending().get()
            || state.merge.pending().get()
            || state.discard.pending().get()
            || state.reset.pending().get()
            || state.resume.pending().get()
            || state.abort.pending().get()
    });
    let subagents_active: Signal<u32> =
        Signal::derive(move || state.transcript.with(|t: &Transcript| t.subagents_active));
    let subagents_list: Signal<Vec<SubagentTaskDto>> = Signal::derive(move || {
        state
            .subagents
            .get()
            .and_then(|r: Result<Vec<SubagentTaskDto>, ServerFnError>| r.ok())
            .unwrap_or_default()
    });
    let subagents_error: Signal<Option<String>> = Signal::derive(move || {
        state
            .subagents
            .get()
            .and_then(|r: Result<Vec<SubagentTaskDto>, ServerFnError>| r.err())
            .map(|e: ServerFnError| e.to_string())
    });
    let on_select_subagent: Callback<String> =
        Callback::new(move |id: String| state.selected_subagent.set(Some(id)));
    let on_close_subagents: Callback<()> = Callback::new(move |_| state.show_subagents.set(false));
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
    let prompt: Memo<String> = Memo::new(move |_| {
        loaded_task.with(|t: &Option<TaskDto>| {
            t.as_ref()
                .map(|t: &TaskDto| t.prompt.clone())
                .unwrap_or_default()
        })
    });

    // The tab shows the task title once it is known.
    let dock: DockContext = use_dock();
    let tab_path: String = format!("/tasks/{}", state.task_id.get_value());
    Effect::new(move |_| {
        if let Some(task) = loaded_task.get() {
            dock.set_title(&tab_path, &task.title);
        }
    });

    Effect::new(move |_| {
        if shows_workspace.get() {
            state.transcript.track();
            follow_bottom(&scroll_id.get_value(), state.autoscroll.get());
        }
    });

    view! {
        <Modal title=SUBAGENTS.get() open=state.show_subagents on_close=on_close_subagents>
            {move || subagents_error.get().map(|e: String| view! { <ErrorText message=e /> })}
            <SubagentsPanel
                subagents=subagents_list
                selected=state.selected_subagent
                on_select=on_select_subagent
            />
        </Modal>
        <Show
            when=move || !state.removed.get()
            fallback=|| {
                view! {
                    <section class="card stack m-4">
                        <h1 class="task-title">{TASK_DELETED.get()}</h1>
                        <p class="muted">{TASK_GONE.get()}</p>
                        <TabLink href="/" class="btn-secondary">
                            {BACK_TO_BOARD.get()}
                        </TabLink>
                    </section>
                }
            }
        >
            <div class="flex h-full min-h-0">
                <div class="flex min-w-0 flex-1 flex-col">
                    <div class="flex shrink-0 items-center gap-2 border-b border-border bg-surface px-4 py-2.5">
                        <div class="min-w-0 flex-1">
                            <Transition fallback=|| view! { <Loading text=LOADING_TASK.get() /> }>
                                {move || Suspend::new(async move {
                                    match state.task.await {
                                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                        Ok(task) => {
                                            let task: TaskDto = task;
                                            let is_backlog: bool = task.stage == TaskStageDto::Backlog;
                                            view! {
                                                <h1 class="page-title truncate text-xl">{task.title.clone()}</h1>
                                                {(!is_backlog)
                                                    .then(|| {
                                                        view! {
                                                            <p class="truncate text-[11px] text-muted">
                                                                {BRANCH_PREFIX.get()}
                                                                <code class="font-mono" title=task.branch.clone()>
                                                                    {short_branch(&task.branch)}
                                                                </code>
                                                                {BRANCH_FROM.get()}
                                                                <code class="font-mono">{task.base_branch.clone()}</code>
                                                            </p>
                                                        }
                                                    })}
                                                {task.error.clone().map(|e: String| view! { <ErrorText message=e /> })}
                                                {task
                                                    .resume_notice()
                                                    .map(|notice: String| view! { <p class="text-xs text-muted">{notice}</p> })}
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
                                {PANEL.get()}
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
                        <div class="group/scroll relative flex min-h-0 flex-1 flex-col" data-at-bottom="true">
                            <div
                                id=scroll_id.get_value()
                                class="stack min-h-0 flex-1 overflow-y-auto p-4"
                            >
                                <TranscriptView transcript=state.transcript />
                            </div>
                            <button
                                class="btn-secondary absolute right-4 bottom-3 rounded-full shadow-md group-data-[at-bottom=true]/scroll:hidden!"
                                aria-label=GO_TO_LAST_MESSAGE.get()
                                on:click=move |_| scroll_to_bottom(&scroll_id.get_value())
                            >
                                {LAST_MESSAGE.get()}
                            </button>
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
                                    {move || if state.connected.get() { LIVE.get() } else { DISCONNECTED.get() }}
                                </span>
                            </div>
                            <Show when=move || can_message.get()>
                                <Composer
                                    message=state.message
                                    attachments=state.images
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
                        width=panel_width
                        label=TASK_PANEL.get()
                    >
                        <div class="flex shrink-0 items-center justify-between border-b border-border px-3 py-2 md:hidden">
                            <span class="text-xs font-bold tracking-widest uppercase">{TASK.get()}</span>
                            <button
                                class="btn-ghost min-h-8 px-2 py-1 text-xs"
                                on:click=move |_| right_overlay.set(false)
                            >
                                {CLOSE.get()}
                            </button>
                        </div>
                        <TabBar
                            tabs=tab_items(&extra_tabs.get_value())
                            active=active_tab
                            on_select=on_select_tab
                        />
                        <div class="stack min-h-0 flex-1 overflow-y-auto p-3">
                            <Show when=move || active_tab.get() == TAB_ACTIONS>
                                <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
                                    {move || Suspend::new(async move {
                                        match state.task.await {
                                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                            Ok(task) => {
                                                let task: TaskDto = task;
                                                view! {
                                                    <TaskActions
                                                        actions=task.actions
                                                        handlers
                                                        busy
                                                        subagents_active
                                                        terminal_href=format!("/tasks/{}/terminal", task.id)
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
                                                                    view! { <p class="notice notice-info animate-fade-in">{MERGED_INTO_BASE.get()}</p> }
                                                                        .into_any()
                                                                }
                                                                MergeOutcomeDto::Conflict { files } => {
                                                                    view! {
                                                                        <p class="notice notice-warn animate-fade-in">
                                                                            {MERGE_CONFLICTS.fmt(&[("files", &files.join(", "))])}
                                                                        </p>
                                                                    }
                                                                        .into_any()
                                                                }
                                                            })
                                                    }}
                                                    <div class="stack gap-1">
                                                        <span class="eyebrow">{BASE_BRANCH.get()}</span>
                                                        <code class="block break-all rounded-md bg-surface-2/60 px-2 py-1 font-mono text-[11px] text-muted">
                                                            {task.base_branch.clone()}
                                                        </code>
                                                    </div>
                                                    <div class="stack gap-1">
                                                        <span class="eyebrow">"Worktree"</span>
                                                        <code class="block break-all rounded-md bg-surface-2/60 px-2 py-1 font-mono text-[11px] text-muted">
                                                            {task.worktree_path.clone()}
                                                        </code>
                                                    </div>
                                                    <label class="flex min-h-9 items-center gap-2 text-xs text-muted">
                                                        <input
                                                            type="checkbox"
                                                            class="h-4 w-4 accent-accent"
                                                            prop:checked=move || state.autoscroll.get()
                                                            on:change=move |ev| state.autoscroll.set(event_target_checked(&ev))
                                                        />
                                                        {AUTO_SCROLL.get()}
                                                    </label>
                                                }
                                                    .into_any()
                                            }
                                        }
                                    })}
                                </Transition>
                                <details class="card prompt">
                                    <summary>{INITIAL_PROMPT.get()}</summary>
                                    <pre class="font-mono text-sm whitespace-pre-wrap">{move || prompt.get()}</pre>
                                </details>
                                <Suspense fallback=|| ()>
                                    {move || Suspend::new(async move {
                                        let items: Vec<TaskAttachmentDto> = state
                                            .attachments
                                            .await
                                            .unwrap_or_default();
                                        view! { <TaskAttachmentsList items /> }
                                    })}
                                </Suspense>
                            </Show>
                            {extra_tabs
                                .get_value()
                                .into_iter()
                                .map(|tab: TaskExtraTab| {
                                    view! {
                                        <Show when=move || active_tab.get() == tab.id>
                                            {move || tab.render.run(state.task_id.get_value())}
                                        </Show>
                                    }
                                })
                                .collect_view()}
                        </div>
                    </CollapsePanel>
                </Show>
            </div>
        </Show>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn tabs_are_actions_plus_the_extra_one() {
        let ids: Vec<&str> = tab_items(&[]).iter().map(|t: &TabItem| t.id).collect();
        assert_eq!(ids, vec![TAB_ACTIONS]);
        Owner::new().with(|| {
            let extra = |id: &'static str| TaskExtraTab {
                id,
                label: id,
                render: Callback::new(|_: String| ().into_any()),
            };
            let ids: Vec<&str> = tab_items(&[extra("files"), extra("code-host")])
                .iter()
                .map(|t: &TabItem| t.id)
                .collect();
            assert_eq!(ids, vec![TAB_ACTIONS, "files", "code-host"]);
        });
    }

    #[test]
    fn short_branch_drops_uuid_suffix() {
        assert_eq!(
            short_branch("pi/fix-login-3f2a9c1e-5b7d-4e8a-9c0b-1a2b3c4d5e6f"),
            "pi/fix-login"
        );
    }

    #[test]
    fn short_branch_keeps_short_plain_names() {
        assert_eq!(short_branch("pi/fix-login"), "pi/fix-login");
        assert_eq!(short_branch("main"), "main");
    }

    #[test]
    fn short_branch_middle_truncates_long_names() {
        let long: String = format!("pi/{}", "a-very-long-descriptive-name".repeat(3));
        let short: String = short_branch(&long);
        assert_eq!(short.chars().count(), BRANCH_MAX_CHARS);
        assert!(short.contains('…'));
        assert!(short.starts_with("pi/"));
        assert!(short.ends_with("name"));
    }
}
