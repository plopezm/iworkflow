use leptos::html;
use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::segmented_control::{SegmentOption, SegmentedControl};
use crate::core::components::toast::ToastStack;
use crate::core::dock::context::{TabContext, use_dock, use_tab_nav};
use crate::core::dock::link::TabLink;
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::board_toolbar::BoardToolbar;
use crate::modules::tasks::presentation::components::loose_tasks_panel::LooseTasksPanel;
use crate::modules::tasks::presentation::components::task_card::{
    TaskActionBusy, TaskCardBusy, TaskCardHandlers,
};
use crate::modules::tasks::presentation::components::task_graph_canvas::{
    TaskGraphCanvas, canvas_size,
};
use crate::modules::tasks::presentation::components::task_modal::TaskModal;
use crate::modules::tasks::presentation::state::board_state::BoardState;
use crate::modules::tasks::presentation::state::graph_model::{
    BoardTab, BoardWorkspace, board_path,
};

const BOARD: Msg = Msg {
    en: "Board",
    es: "Tablero",
    zh: "看板",
    de: "Board",
    fr: "Tableau",
};

const LOADING_BOARD: Msg = Msg {
    en: "Loading board…",
    es: "Cargando tablero…",
    zh: "正在加载看板…",
    de: "Board wird geladen…",
    fr: "Chargement du tableau…",
};

const EMPTY_TITLE: Msg = Msg {
    en: "No tasks in this workspace yet",
    es: "Aún no hay tareas en este workspace",
    zh: "此工作区还没有任务",
    de: "In diesem Workspace gibt es noch keine Aufgaben",
    fr: "Aucune tâche dans cet espace de travail pour l'instant",
};

const EMPTY_HINT: Msg = Msg {
    en: "Create the first task, then chain more tasks to it: a task starts when the ones it depends on are merged.",
    es: "Crea la primera tarea y encadena más: una tarea empieza cuando se integran aquellas de las que depende.",
    zh: "创建第一个任务，然后串联更多任务：当所依赖的任务合并后，该任务才会开始。",
    de: "Erstelle die erste Aufgabe und verkette weitere: Eine Aufgabe startet, sobald die Aufgaben, von denen sie abhängt, integriert sind.",
    fr: "Créez la première tâche, puis enchaînez-en d'autres : une tâche démarre quand celles dont elle dépend sont intégrées.",
};

const CREATE_FIRST_TASK: Msg = Msg {
    en: "Create the first task",
    es: "Crear la primera tarea",
    zh: "创建第一个任务",
    de: "Erste Aufgabe erstellen",
    fr: "Créer la première tâche",
};

const NO_WORKSPACES: Msg = Msg {
    en: "Register a workspace to start planning tasks.",
    es: "Registra un workspace para empezar a planificar tareas.",
    zh: "注册一个工作区以开始规划任务。",
    de: "Registriere einen Workspace, um Aufgaben zu planen.",
    fr: "Enregistrez un espace de travail pour planifier des tâches.",
};

const GO_TO_WORKSPACES: Msg = Msg {
    en: "Go to Workspaces",
    es: "Ir a Workspaces",
    zh: "前往工作区",
    de: "Zu den Workspaces",
    fr: "Aller aux espaces de travail",
};

const WORKSPACE_NOT_FOUND: Msg = Msg {
    en: "This workspace no longer exists. Pick another one above.",
    es: "Este workspace ya no existe. Elige otro arriba.",
    zh: "此工作区已不存在。请在上方选择其他工作区。",
    de: "Dieser Workspace existiert nicht mehr. Wähle oben einen anderen.",
    fr: "Cet espace de travail n'existe plus. Choisissez-en un autre ci-dessus.",
};

const LOAD_MORE_DONE: Msg = Msg {
    en: "Load more completed",
    es: "Cargar más terminadas",
    zh: "加载更多已完成",
    de: "Mehr erledigte laden",
    fr: "Charger plus de terminées",
};

const TAB_LOOSE: Msg = Msg {
    en: "Loose ({n})",
    es: "Sueltas ({n})",
    zh: "零散 ({n})",
    de: "Lose ({n})",
    fr: "Libres ({n})",
};

const TAB_GRAPH: Msg = Msg {
    en: "Graph",
    es: "Grafo",
    zh: "图",
    de: "Graph",
    fr: "Graphe",
};

const TABS_LABEL: Msg = Msg {
    en: "Board view",
    es: "Vista del tablero",
    zh: "看板视图",
    de: "Board-Ansicht",
    fr: "Vue du tableau",
};

/// Opciones de las subpestañas del tablero (pantallas estrechas).
pub fn tab_options(loose_count: usize) -> Vec<SegmentOption> {
    vec![
        SegmentOption {
            value: BoardTab::Loose.as_str().to_string(),
            label: TAB_LOOSE.fmt(&[("n", &loose_count.to_string())]),
        },
        SegmentOption {
            value: BoardTab::Graph.as_str().to_string(),
            label: TAB_GRAPH.get().to_string(),
        },
    ]
}

/// Qué muestra el tablero sobre la carga de sus datos; los tres casos son excluyentes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LoadStatus {
    /// Datos listos: no se muestra nada.
    Ready,
    /// Aún sin datos y sin error: texto de carga.
    Loading,
    /// Al menos una carga falló: mensajes distintos, sin repetidos y sin texto de carga.
    Failed(Vec<String>),
}

/// Decide el estado de carga del tablero a partir de lo cargado y de los errores del grafo y
/// de las tareas (función pura). Un error siempre prevalece sobre el texto de carga.
pub fn load_status(
    graph_loaded: bool,
    tasks_loaded: bool,
    errors: [Option<String>; 2],
) -> LoadStatus {
    let mut messages: Vec<String> = Vec::new();
    for message in errors.into_iter().flatten() {
        if !messages.contains(&message) {
            messages.push(message);
        }
    }
    if !messages.is_empty() {
        LoadStatus::Failed(messages)
    } else if graph_loaded && tasks_loaded {
        LoadStatus::Ready
    } else {
        LoadStatus::Loading
    }
}

/// Tamaño del lienzo con el que encuadrar, solo si los datos están cargados, la pestaña se
/// ve y el lienzo tiene área (un lienzo oculto o colapsado mide 0×0 y encuadraría mal).
pub fn fit_size(ready: bool, visible: bool, size: Option<(f64, f64)>) -> Option<(f64, f64)> {
    size.filter(|(width, height): &(f64, f64)| ready && visible && *width > 0.0 && *height > 0.0)
}

/// Título de la pestaña del tablero de un workspace (función pura).
pub fn board_tab_title(workspace_name: &str) -> String {
    format!("{} · {workspace_name}", BOARD.get())
}

/// Nombre del workspace `id` entre los listados.
fn workspace_name(workspaces: &[WorkspaceOptionDto], id: &str) -> Option<String> {
    workspaces
        .iter()
        .find(|w: &&WorkspaceOptionDto| w.id == id)
        .map(|w: &WorkspaceOptionDto| w.name.clone())
}

/// Tablero de un workspace: toolbar con selector de workspace y lienzo con el grafo de tareas.
#[component]
pub fn BoardPage() -> impl IntoView {
    let state: BoardState = BoardState::new();
    state.watch_action_errors();
    let nav = use_tab_nav();
    let dock = use_dock();
    let canvas: NodeRef<html::Div> = NodeRef::new();

    // Cierra el modal tras crear con éxito (estado de UI, sin lógica de negocio).
    Effect::new(move |_| {
        if matches!(state.create.value().get(), Some(Ok(_))) {
            state.close_modal();
        }
    });

    let handlers: TaskCardHandlers = TaskCardHandlers {
        start: Callback::new(move |id: String| state.start_task(id)),
        edit: Callback::new(move |task: TaskDto| {
            nav.open(&format!("/tasks/{}/edit", task.id));
        }),
        delete: Callback::new(move |id: String| state.delete_task(id)),
        finish: Callback::new(move |id: String| state.finish_task(id)),
        merge: Callback::new(move |id: String| state.merge_task(id)),
    };
    // Cada acción bloquea solo el nodo al que se aplicó, no el tablero entero.
    let busy: TaskCardBusy = TaskCardBusy {
        start: TaskActionBusy {
            target: state.start_target,
            pending: state.start.pending().into(),
        },
        delete: TaskActionBusy {
            target: state.delete_target,
            pending: state.delete.pending().into(),
        },
        finish: TaskActionBusy {
            target: state.finish_target,
            pending: state.finish.pending().into(),
        },
        merge: TaskActionBusy {
            target: state.merge_target,
            pending: state.merge.pending().into(),
        },
    };

    // La pestaña lleva el nombre del workspace que muestra.
    let selected_name: Memo<Option<String>> = Memo::new(move |_| {
        let id: String = state.workspace_id.get()?;
        state
            .workspace_options
            .with(|all: &Vec<WorkspaceOptionDto>| workspace_name(all, &id))
    });
    let tab: Option<TabContext> = use_context::<TabContext>();
    let tab_path: Option<String> = tab.as_ref().map(|t: &TabContext| t.path.clone());
    Effect::new(move |_| {
        if let (Some(tab), Some(name)) = (tab.as_ref(), selected_name.get()) {
            dock.set_title(&tab.path, &board_tab_title(&name));
        }
    });
    // Encuadra el grafo una vez, cuando llegan los datos y el lienzo es visible y tiene área.
    // La visibilidad de la pestaña se sigue para reintentar al volver a ella.
    Effect::new(move |fitted: Option<bool>| {
        if fitted == Some(true) {
            return true;
        }
        let ready: bool = state.graph_loaded.get() && state.store.loaded.get();
        let visible: bool = tab_path
            .as_deref()
            .is_none_or(|path: &str| dock.is_path_visible(path));
        let size: Option<(f64, f64)> = canvas.get().and_then(|_| canvas_size(canvas));
        if let Some(size) = fit_size(ready, visible, size) {
            state.fit(size);
            return true;
        }
        false
    });

    let on_fit: Callback<()> = Callback::new(move |_| {
        if let Some(size) = canvas_size(canvas) {
            state.fit(size);
        }
    });
    let load: Memo<LoadStatus> = Memo::new(move |_| {
        load_status(
            state.graph_loaded.get(),
            state.store.loaded.get(),
            [
                state
                    .graph
                    .get()
                    .and_then(Result::err)
                    .map(|e: ServerFnError| e.to_string()),
                state
                    .store
                    .tasks
                    .get()
                    .and_then(Result::err)
                    .map(|e: ServerFnError| e.to_string()),
            ],
        )
    });
    let is_empty: Memo<bool> = Memo::new(move |_| {
        state.graph_loaded.get()
            && state.store.loaded.get()
            && state.view.with(|g| g.nodes.is_empty())
            && state.loose.with(Vec::is_empty)
    });

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <BoardToolbar
                workspaces=state.workspace_options
                selected_workspace=state.workspace_id
                show_completed=state.show_completed
                on_select_workspace=Callback::new(move |id: String| {
                    nav.replace(&board_path(&id));
                })
                on_fit
                on_new_task=Callback::new(move |_| state.open_create())
            />
            <div class="board-body">
                {move || match state.workspace.get() {
                    BoardWorkspace::Loading => {
                        view! { <p class="muted p-4">{LOADING_BOARD.get()}</p> }.into_any()
                    }
                    BoardWorkspace::Empty => {
                        view! {
                            <div class="stack items-center gap-3 p-8 text-center">
                                <p class="muted">{NO_WORKSPACES.get()}</p>
                                <TabLink href="/workspaces" class="btn-secondary">
                                    {GO_TO_WORKSPACES.get()}
                                </TabLink>
                            </div>
                        }
                            .into_any()
                    }
                    BoardWorkspace::Missing => {
                        view! { <p class="muted p-8 text-center">{WORKSPACE_NOT_FOUND.get()}</p> }
                            .into_any()
                    }
                    BoardWorkspace::Selected(_) => {
                        view! {
                            <SegmentedControl
                                class="board-tabs"
                                options=Signal::derive(move || {
                                    tab_options(state.loose.with(Vec::len))
                                })
                                selected=Signal::derive(move || {
                                    state.active_tab.get().as_str().to_string()
                                })
                                aria_label=TABS_LABEL.get()
                                on_select=Callback::new(move |value: String| {
                                    if let Some(tab) = BoardTab::from_value(&value) {
                                        state.select_tab(tab);
                                    }
                                })
                            />
                            <div
                                class="board-panes"
                                data-tab=move || state.active_tab.get().as_str()
                            >
                            <LooseTasksPanel state canvas handlers busy />
                            <div class="board-canvas">
                            <TaskGraphCanvas state canvas handlers busy />
                            <Show when=move || load.get() == LoadStatus::Loading>
                                <p class="muted pointer-events-none absolute left-4 top-3">
                                    {LOADING_BOARD.get()}
                                </p>
                            </Show>
                            <Show when=move || is_empty.get()>
                                <div class="pointer-events-none absolute inset-0 flex items-center justify-center p-6">
                                    <div class="graph-empty pointer-events-auto">
                                        <h2 class="text-sm font-semibold text-fg">
                                            {EMPTY_TITLE.get()}
                                        </h2>
                                        <p class="muted max-w-sm">{EMPTY_HINT.get()}</p>
                                        <button
                                            type="button"
                                            class="btn-primary"
                                            on:click=move |_| state.open_create()
                                        >
                                            {CREATE_FIRST_TASK.get()}
                                        </button>
                                    </div>
                                </div>
                            </Show>
                            <Show when=move || state.done_has_more.get()>
                                <button
                                    type="button"
                                    class="btn-secondary absolute bottom-3 right-3"
                                    on:click=move |_| state.load_more_done()
                                >
                                    {LOAD_MORE_DONE.get()}
                                </button>
                            </Show>
                            </div>
                            </div>
                        }
                            .into_any()
                    }
                }}
                {move || match load.get() {
                    LoadStatus::Failed(messages) => {
                        messages
                            .into_iter()
                            .map(|message: String| view! { <ErrorText message /> })
                            .collect_view()
                            .into_any()
                    }
                    LoadStatus::Ready | LoadStatus::Loading => ().into_any(),
                }}
            </div>
            // Fuera de cualquier `Suspend`: una recarga de workspaces no debe reconstruir el modal
            // (perdería el foco y lo escrito en el formulario).
            <TaskModal state workspaces=state.workspace_options.into() />
            <ToastStack toasts=state.toasts />
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn workspace(id: &str, name: &str) -> WorkspaceOptionDto {
        WorkspaceOptionDto {
            id: id.to_string(),
            name: name.to_string(),
            base_branch: "main".to_string(),
        }
    }

    #[test]
    fn tab_options_carry_the_loose_count() {
        let options: Vec<SegmentOption> = tab_options(3);
        assert_eq!(options[0].value, "loose");
        assert_eq!(options[0].label, "Loose (3)");
        assert_eq!(options[1].value, "graph");
    }

    #[test]
    fn errors_win_over_the_loading_text_and_are_not_repeated() {
        let same: String = "sin conexión".to_string();
        assert_eq!(
            load_status(false, false, [Some(same.clone()), Some(same.clone())]),
            LoadStatus::Failed(vec![same.clone()])
        );
        assert_eq!(
            load_status(true, true, [None, Some("tareas".to_string())]),
            LoadStatus::Failed(vec!["tareas".to_string()])
        );
        assert_eq!(
            load_status(
                false,
                false,
                [Some("grafo".to_string()), Some(same.clone())]
            ),
            LoadStatus::Failed(vec!["grafo".to_string(), same])
        );
    }

    #[test]
    fn without_errors_the_board_is_loading_until_both_loads_finish() {
        assert_eq!(load_status(false, true, [None, None]), LoadStatus::Loading);
        assert_eq!(load_status(true, false, [None, None]), LoadStatus::Loading);
        assert_eq!(load_status(true, true, [None, None]), LoadStatus::Ready);
    }

    #[test]
    fn fit_waits_for_data_visibility_and_a_non_empty_canvas() {
        let size: Option<(f64, f64)> = Some((800.0, 600.0));
        assert_eq!(fit_size(true, true, size), size);
        assert_eq!(fit_size(false, true, size), None);
        assert_eq!(fit_size(true, false, size), None);
        assert_eq!(fit_size(true, true, None), None);
        assert_eq!(fit_size(true, true, Some((0.0, 600.0))), None);
        assert_eq!(fit_size(true, true, Some((800.0, 0.0))), None);
    }

    #[test]
    fn tab_title_includes_the_workspace_name() {
        assert_eq!(board_tab_title("api"), "Board · api");
    }

    #[test]
    fn workspace_name_is_looked_up_by_id() {
        let all: Vec<WorkspaceOptionDto> = vec![workspace("w1", "api"), workspace("w2", "web")];
        assert_eq!(workspace_name(&all, "w2").as_deref(), Some("web"));
        assert_eq!(workspace_name(&all, "zz"), None);
    }
}
