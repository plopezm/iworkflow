use leptos::ev::PointerEvent;
use leptos::prelude::*;

use crate::core::dock::link::TabLink;
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskPriorityDto,
};
use crate::modules::tasks::presentation::components::stage_badge::StageBadge;
use crate::modules::tasks::presentation::components::task_card::{
    TaskCardActions, TaskCardBusy, TaskCardHandlers, pending_dialogs_label, stage_rail_class,
};
use crate::modules::tasks::presentation::state::graph_geometry::{NODE_HEIGHT, NODE_WIDTH, Point};
use crate::modules::tasks::presentation::state::graph_model::NodeView;

const ADD_NEXT: Msg = Msg {
    en: "Add next task",
    es: "Añadir tarea siguiente",
    zh: "添加后续任务",
    de: "Nächste Aufgabe hinzufügen",
    fr: "Ajouter la tâche suivante",
};

const ADD_PARALLEL: Msg = Msg {
    en: "Add parallel task",
    es: "Añadir tarea en paralelo",
    zh: "添加并行任务",
    de: "Parallele Aufgabe hinzufügen",
    fr: "Ajouter une tâche parallèle",
};

const NEXT_SHORT: Msg = Msg {
    en: "Next",
    es: "Siguiente",
    zh: "后续",
    de: "Nächste",
    fr: "Suivante",
};

const PARALLEL_SHORT: Msg = Msg {
    en: "Parallel",
    es: "Paralela",
    zh: "并行",
    de: "Parallel",
    fr: "Parallèle",
};

const DETACH_SHORT: Msg = Msg {
    en: "Remove from graph",
    es: "Quitar del grafo",
    zh: "移出图表",
    de: "Aus Graph entfernen",
    fr: "Retirer du graphe",
};

const DETACH_TITLE: Msg = Msg {
    en: "Remove from graph: deletes all its connections and sends it back to the loose tasks",
    es: "Quitar del grafo: elimina todas sus conexiones y la devuelve a las tareas sueltas",
    zh: "移出图表：删除其所有连线，并将其放回零散任务",
    de: "Aus Graph entfernen: löscht alle Verbindungen und legt die Aufgabe zu den losen Aufgaben zurück",
    fr: "Retirer du graphe : supprime toutes ses connexions et la renvoie aux tâches libres",
};

const WAITING_ONE: Msg = Msg {
    en: "Waiting for 1 predecessor",
    es: "Esperando 1 predecesora",
    zh: "等待 1 个前置任务",
    de: "Wartet auf 1 Vorgänger",
    fr: "En attente de 1 prédécesseur",
};

const WAITING_MANY: Msg = Msg {
    en: "Waiting for {n} predecessors",
    es: "Esperando {n} predecesoras",
    zh: "等待 {n} 个前置任务",
    de: "Wartet auf {n} Vorgänger",
    fr: "En attente de {n} prédécesseurs",
};

const INPUT_PORT: Msg = Msg {
    en: "Input: drop a connection here",
    es: "Entrada: suelta aquí una conexión",
    zh: "输入：将连线拖到此处",
    de: "Eingang: Verbindung hier ablegen",
    fr: "Entrée : déposez une connexion ici",
};

const OUTPUT_PORT: Msg = Msg {
    en: "Output: drag to another task to make it wait for this one",
    es: "Salida: arrastra a otra tarea para que espere a esta",
    zh: "输出：拖到另一个任务，让它等待此任务",
    de: "Ausgang: auf eine andere Aufgabe ziehen, damit sie auf diese wartet",
    fr: "Sortie : glissez vers une autre tâche pour qu'elle attende celle-ci",
};

/// Texto del indicador «esperando N predecesoras»; `None` si no espera a ninguna.
pub fn waiting_label(waiting: usize) -> Option<String> {
    match waiting {
        0 => None,
        1 => Some(WAITING_ONE.get().to_string()),
        n => Some(WAITING_MANY.fmt(&[("n", &n.to_string())])),
    }
}

/// Valor de `style` que coloca un nodo en el lienzo.
pub fn node_style(position: Point) -> String {
    format!(
        "transform: translate({:.1}px, {:.1}px); width: {NODE_WIDTH}px; height: {NODE_HEIGHT}px;",
        position.x, position.y
    )
}

/// Clases del nodo según su selección y si es el destino de una conexión en curso.
pub fn node_class(selected: bool, connect_target: bool, needs_attention: bool) -> String {
    let mut class: String = "graph-node group".to_string();
    if selected {
        class.push_str(" graph-node-selected");
    }
    if connect_target {
        class.push_str(" graph-node-target");
    }
    if needs_attention {
        class.push_str(" graph-node-attention");
    }
    class
}

/// Nodo del tablero: tarjeta compacta de una tarea con puertos de entrada y salida y
/// accesos para añadir una tarea siguiente o paralela.
#[component]
pub fn TaskNode(
    node: NodeView,
    #[prop(into)] position: Signal<Point>,
    #[prop(into)] selected: Signal<bool>,
    #[prop(into)] connect_target: Signal<bool>,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
    /// Empieza a arrastrar el nodo.
    on_drag_start: Callback<PointerEvent>,
    /// Empieza una arista desde el puerto de salida.
    on_connect_start: Callback<PointerEvent>,
    on_add_next: Callback<String>,
    on_add_parallel: Callback<String>,
    /// Saca la tarea del grafo y la devuelve a las sueltas.
    on_detach: Callback<String>,
) -> impl IntoView {
    let task: TaskDto = node.task;
    let id: String = task.id.clone();
    let next_id: String = id.clone();
    let parallel_id: String = id.clone();
    let detach_id: String = id.clone();
    let needs_attention: bool = task.needs_attention;
    let title: String = task.title.clone();
    let article_class: String = format!(
        "card animate-fade-in flex h-full flex-col gap-1.5 !p-2.5 {}",
        stage_rail_class(task.stage)
    );

    view! {
        <div
            class=move || node_class(selected.get(), connect_target.get(), needs_attention)
            style=move || node_style(position.get())
        >
            <span class="graph-port graph-port-in" title=INPUT_PORT.get() aria-hidden="true"></span>
            <article
                class=article_class
                on:pointerdown=move |ev: PointerEvent| on_drag_start.run(ev)
            >
                <div class="flex items-start justify-between gap-2">
                    <TabLink
                        href=format!("/tasks/{id}")
                        class="line-clamp-2 min-w-0 text-[13px] font-medium leading-snug text-fg transition-colors hover:text-accent"
                    >
                        <span on:pointerdown=|ev: PointerEvent| ev.stop_propagation()>
                            {title}
                        </span>
                    </TabLink>
                    <StageBadge stage=task.stage />
                </div>
                <div class="flex min-h-5 flex-wrap items-center gap-1">
                    {(task.priority != TaskPriorityDto::Normal)
                        .then(|| {
                            view! {
                                <span class=format!(
                                    "badge badge-{}",
                                    task.priority.variant(),
                                )>{task.priority.label()}</span>
                            }
                        })}
                    {waiting_label(node.waiting)
                        .map(|label: String| {
                            view! {
                                <span class="badge badge-muted gap-1">
                                    <span class="graph-waiting-dot" aria-hidden="true"></span>
                                    {label}
                                </span>
                            }
                        })}
                    {(task.pending_dialogs > 0)
                        .then(|| {
                            view! {
                                <span class="badge badge-muted">
                                    {pending_dialogs_label(task.pending_dialogs)}
                                </span>
                            }
                        })}
                </div>
                {task
                    .error
                    .clone()
                    .map(|e: String| {
                        let full: String = e.clone();
                        view! { <p class="truncate text-xs text-error" title=full>{e}</p> }
                    })}
                {task
                    .resume_notice()
                    .map(|notice: String| view! { <p class="truncate text-xs text-muted">{notice}</p> })}
                <div
                    class="mt-auto flex flex-wrap items-center gap-1"
                    on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
                >
                    <TaskCardActions task=task.clone() handlers busy />
                </div>
            </article>
            <button
                type="button"
                class="graph-port graph-port-out"
                title=OUTPUT_PORT.get()
                aria-label=OUTPUT_PORT.get()
                on:pointerdown=move |ev: PointerEvent| {
                    ev.stop_propagation();
                    on_connect_start.run(ev);
                }
            ></button>
            <div
                class="graph-node-tools"
                on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
            >
                <button
                    type="button"
                    class="graph-tool"
                    title=ADD_NEXT.get()
                    aria-label=ADD_NEXT.get()
                    on:click=move |_| on_add_next.run(next_id.clone())
                >
                    <span aria-hidden="true">"→+"</span>
                    {NEXT_SHORT.get()}
                </button>
                <button
                    type="button"
                    class="graph-tool"
                    title=ADD_PARALLEL.get()
                    aria-label=ADD_PARALLEL.get()
                    on:click=move |_| on_add_parallel.run(parallel_id.clone())
                >
                    <span aria-hidden="true">"⇉"</span>
                    {PARALLEL_SHORT.get()}
                </button>
                <button
                    type="button"
                    class="graph-tool"
                    title=DETACH_TITLE.get()
                    aria-label=DETACH_TITLE.get()
                    on:click=move |_| on_detach.run(detach_id.clone())
                >
                    <span aria-hidden="true">"⊖"</span>
                    {DETACH_SHORT.get()}
                </button>
            </div>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn waiting_label_is_hidden_without_predecessors_and_pluralises() {
        assert_eq!(waiting_label(0), None);
        assert_eq!(
            waiting_label(1).as_deref(),
            Some("Waiting for 1 predecessor")
        );
        assert_eq!(
            waiting_label(3).as_deref(),
            Some("Waiting for 3 predecessors")
        );
        assert_eq!(
            WAITING_MANY.in_locale(Locale::Es).replace("{n}", "2"),
            "Esperando 2 predecesoras"
        );
    }

    #[test]
    fn node_style_places_the_node_with_a_fixed_size() {
        assert_eq!(
            node_style(Point::new(10.0, 20.5)),
            "transform: translate(10.0px, 20.5px); width: 264px; height: 140px;"
        );
    }

    #[test]
    fn node_class_reflects_selection_target_and_attention() {
        assert_eq!(node_class(false, false, false), "graph-node group");
        let class: String = node_class(true, true, true);
        assert!(class.contains("graph-node-selected"));
        assert!(class.contains("graph-node-target"));
        assert!(class.contains("graph-node-attention"));
    }
}
