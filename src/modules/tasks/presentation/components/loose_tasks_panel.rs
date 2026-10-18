use leptos::html;
use leptos::prelude::*;

use crate::core::components::collapsible_panel::CollapsiblePanel;
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskDto;
use crate::modules::tasks::presentation::components::loose_task_card::LooseTaskCard;
use crate::modules::tasks::presentation::components::task_card::{TaskCardBusy, TaskCardHandlers};
use crate::modules::tasks::presentation::components::task_graph_canvas::{
    canvas_rect, canvas_size, window_width,
};
use crate::modules::tasks::presentation::state::board_state::BoardState;
use crate::modules::tasks::presentation::state::graph_geometry::Point;
use crate::modules::tasks::presentation::state::loose_drag::{LooseDrag, is_narrow_viewport};

const TITLE: Msg = Msg {
    en: "Loose tasks",
    es: "Tareas sueltas",
    zh: "零散任务",
    de: "Lose Aufgaben",
    fr: "Tâches libres",
};

const EXPAND: Msg = Msg {
    en: "Show loose tasks",
    es: "Mostrar tareas sueltas",
    zh: "显示零散任务",
    de: "Lose Aufgaben anzeigen",
    fr: "Afficher les tâches libres",
};

const COLLAPSE: Msg = Msg {
    en: "Hide loose tasks",
    es: "Ocultar tareas sueltas",
    zh: "隐藏零散任务",
    de: "Lose Aufgaben ausblenden",
    fr: "Masquer les tâches libres",
};

const EMPTY_TITLE: Msg = Msg {
    en: "No loose tasks",
    es: "No hay tareas sueltas",
    zh: "没有零散任务",
    de: "Keine losen Aufgaben",
    fr: "Aucune tâche libre",
};

const EMPTY_HINT: Msg = Msg {
    en: "Tasks created with “New task” show up here. Run them on their own or drag them onto the canvas to chain them.",
    es: "Las tareas creadas con «Nueva tarea» aparecen aquí. Ejecútalas por separado o arrástralas al lienzo para encadenarlas.",
    zh: "通过“新建任务”创建的任务会显示在这里。可单独运行，或拖到画布上进行串联。",
    de: "Mit „Neue Aufgabe“ erstellte Aufgaben erscheinen hier. Führe sie einzeln aus oder ziehe sie auf die Zeichenfläche, um sie zu verketten.",
    fr: "Les tâches créées avec « Nouvelle tâche » apparaissent ici. Exécutez-les seules ou glissez-les sur le canevas pour les enchaîner.",
};

/// Título de la tarea que se arrastra, para el fantasma que sigue al puntero.
fn dragged_title(tasks: &[TaskDto], drag: &LooseDrag) -> Option<String> {
    tasks
        .iter()
        .find(|t: &&TaskDto| t.id == drag.task_id)
        .map(|t: &TaskDto| t.title.clone())
}

/// Clave de una tarjeta suelta para el `<For>`: solo contiene lo que la tarjeta pinta. No
/// incluye el estado del arrastre (puntero, tarea arrastrada) ni `updated_at`, que cambia sin
/// que la tarjeta lo muestre: reconstruir el nodo en pleno arrastre perdería la captura del
/// puntero.
pub fn loose_card_key(task: &TaskDto) -> String {
    format!(
        "{}|{}|{:?}|{:?}|{:?}|{:?}|{:?}|{:?}|{}|{}",
        task.id,
        task.title,
        task.stage,
        task.status,
        task.priority,
        task.error,
        task.resume_at,
        task.actions,
        task.pending_dialogs,
        task.needs_attention,
    )
}

/// Estilo del fantasma de arrastre, anclado a la ventana en la posición del puntero.
pub fn ghost_style(pointer: Point) -> String {
    format!("left: {:.0}px; top: {:.0}px;", pointer.x, pointer.y)
}

/// Panel izquierdo con las tareas que aún no forman parte del grafo: se pueden ejecutar tal
/// cual o arrastrar al lienzo para añadirlas.
#[component]
pub fn LooseTasksPanel(
    state: BoardState,
    canvas: NodeRef<html::Div>,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
) -> impl IntoView {
    let count: Signal<usize> = Signal::derive(move || state.loose.with(Vec::len));
    let on_end: Callback<Point> = Callback::new(move |client: Point| {
        // Arrastrar al lienzo es solo de escritorio; en estrecho se usa el botón.
        let narrow: bool = window_width().is_some_and(is_narrow_viewport);
        state.loose_pointer_up(client, canvas_rect(canvas).filter(|_| !narrow));
    });
    let on_add: Callback<String> = Callback::new(move |id: String| {
        // Sin lienzo medido se usa un tamaño por defecto en vez de ignorar el clic.
        state.add_loose_to_graph(id, canvas_size(canvas).unwrap_or_default());
    });
    let ghost: Memo<Option<(String, Point)>> = Memo::new(move |_| {
        let drag: LooseDrag = state.loose_drag.get()?;
        let title: String = state
            .loose
            .with(|tasks: &Vec<TaskDto>| dragged_title(tasks, &drag))?;
        Some((title, drag.pointer))
    });

    view! {
        <CollapsiblePanel
            title=TITLE.get()
            count
            collapsed=state.panel_collapsed
            expand_label=EXPAND.get()
            collapse_label=COLLAPSE.get()
        >
            <Show
                when=move || count.get() != 0
                fallback=|| {
                    view! {
                        <div class="stack gap-1 p-3 text-center">
                            <p class="text-sm font-medium text-fg">{EMPTY_TITLE.get()}</p>
                            <p class="muted text-xs">{EMPTY_HINT.get()}</p>
                        </div>
                    }
                }
            >
                <ul class="stack gap-2 p-2">
                    <For
                        each=move || state.loose.get()
                        key=loose_card_key
                        children=move |task: TaskDto| {
                            let id: String = task.id.clone();
                            view! {
                                <li>
                                    <LooseTaskCard
                                        task
                                        handlers
                                        busy
                                        on_drag_start=Callback::new(move |p: Point| {
                                            state.loose_pointer_down(id.clone(), p);
                                        })
                                        on_drag_move=Callback::new(move |p: Point| state.loose_pointer_move(p))
                                        on_drag_end=on_end
                                        on_drag_cancel=Callback::new(move |_| state.loose_pointer_cancel())
                                        on_add_to_graph=on_add
                                    />
                                </li>
                            }
                        }
                    />
                </ul>
            </Show>
        </CollapsiblePanel>
        {move || {
            ghost
                .get()
                .map(|(title, pointer): (String, Point)| {
                    view! {
                        <div class="loose-ghost" style=ghost_style(pointer) aria-hidden="true">
                            {title}
                        </div>
                    }
                })
        }}
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
        TaskStageDto, TaskStatusDto,
    };

    fn task(id: &str) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: "w".to_string(),
            title: "título".to_string(),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: TaskStatusDto::Backlog,
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
            actions: Default::default(),
            stage: TaskStageDto::Backlog,
            needs_attention: false,
            pending_dialogs: 0,
            created_at: "1".to_string(),
            updated_at: "1".to_string(),
        }
    }

    #[test]
    fn the_card_key_ignores_what_the_card_does_not_draw() {
        let base: TaskDto = task("a");
        let mut touched: TaskDto = base.clone();
        touched.updated_at = "2".to_string();
        touched.prompt = "otro".to_string();
        assert_eq!(loose_card_key(&base), loose_card_key(&touched));
    }

    #[test]
    fn the_card_key_changes_with_what_the_card_draws() {
        let base: TaskDto = task("a");
        let before: String = loose_card_key(&base);
        let mut renamed: TaskDto = base.clone();
        renamed.title = "nuevo".to_string();
        assert_ne!(before, loose_card_key(&renamed));
        let mut attention: TaskDto = base.clone();
        attention.needs_attention = true;
        assert_ne!(before, loose_card_key(&attention));
        let mut failed: TaskDto = base;
        failed.error = Some("falló".to_string());
        assert_ne!(before, loose_card_key(&failed));
    }

    #[test]
    fn ghost_follows_the_pointer() {
        assert_eq!(
            ghost_style(Point::new(120.4, 80.6)),
            "left: 120px; top: 81px;"
        );
    }
}
