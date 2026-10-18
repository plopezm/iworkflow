use leptos::ev::PointerEvent;
use leptos::html;
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
use crate::modules::tasks::presentation::state::graph_geometry::Point;

const ADD_TO_GRAPH: Msg = Msg {
    en: "Add to graph",
    es: "Añadir al grafo",
    zh: "添加到图",
    de: "Zum Graph hinzufügen",
    fr: "Ajouter au graphe",
};

const DRAG_HINT: Msg = Msg {
    en: "Drag onto the canvas to add it to the graph",
    es: "Arrastra al lienzo para añadirla al grafo",
    zh: "拖到画布上即可添加到图中",
    de: "Auf die Zeichenfläche ziehen, um sie zum Graph hinzuzufügen",
    fr: "Glissez sur le canevas pour l'ajouter au graphe",
};

/// Punto en coordenadas de ventana de un evento de puntero.
pub fn client_point(ev: &PointerEvent) -> Point {
    Point::new(f64::from(ev.client_x()), f64::from(ev.client_y()))
}

/// Tarjeta compacta de una tarea suelta: mismas acciones que un nodo, sin puertos. Se puede
/// arrastrar al lienzo o añadir al grafo con el botón.
#[component]
pub fn LooseTaskCard(
    task: TaskDto,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
    /// El puntero se pulsó sobre la tarjeta (inicio del arrastre).
    on_drag_start: Callback<Point>,
    on_drag_move: Callback<Point>,
    on_drag_end: Callback<Point>,
    on_drag_cancel: Callback<()>,
    on_add_to_graph: Callback<String>,
) -> impl IntoView {
    let card: NodeRef<html::Article> = NodeRef::new();
    let id: String = task.id.clone();
    let add_id: String = id.clone();
    let title: String = task.title.clone();
    let class: String = format!(
        "card loose-card animate-fade-in flex flex-col gap-1.5 !p-2.5 {}",
        stage_rail_class(task.stage)
    );
    let needs_attention: bool = task.needs_attention;

    view! {
        <article
            node_ref=card
            class=class
            class:loose-card-attention=needs_attention
            title=DRAG_HINT.get()
            on:pointerdown=move |ev: PointerEvent| {
                if ev.button() != 0 {
                    return;
                }
                if let Some(el) = card.get_untracked() {
                    el.set_pointer_capture(ev.pointer_id()).ok();
                }
                on_drag_start.run(client_point(&ev));
            }
            on:pointermove=move |ev: PointerEvent| on_drag_move.run(client_point(&ev))
            on:pointerup=move |ev: PointerEvent| on_drag_end.run(client_point(&ev))
            on:pointercancel=move |_| on_drag_cancel.run(())
        >
            <div class="flex items-start justify-between gap-2">
                <TabLink
                    href=format!("/tasks/{id}")
                    class="line-clamp-2 min-w-0 text-[13px] font-medium leading-snug text-fg transition-colors hover:text-accent"
                >
                    <span on:pointerdown=|ev: PointerEvent| ev.stop_propagation()>{title}</span>
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
                class="flex flex-wrap items-center gap-1"
                on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
            >
                <TaskCardActions task=task.clone() handlers busy />
                <button
                    type="button"
                    class="graph-tool ml-auto"
                    on:click=move |_| on_add_to_graph.run(add_id.clone())
                >
                    <span aria-hidden="true">"＋"</span>
                    {ADD_TO_GRAPH.get()}
                </button>
            </div>
        </article>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn add_to_graph_is_translated() {
        assert_eq!(ADD_TO_GRAPH.in_locale(Locale::En), "Add to graph");
        assert_eq!(ADD_TO_GRAPH.in_locale(Locale::Fr), "Ajouter au graphe");
    }
}
