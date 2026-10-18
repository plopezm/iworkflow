use leptos::ev::{PointerEvent, WheelEvent};
use leptos::html;
use leptos::prelude::*;

use crate::core::components::zoom_controls::ZoomControls;
use crate::core::i18n::Msg;
use crate::modules::tasks::presentation::components::graph_edge::{GraphArrowDefs, GraphEdgeLine};
use crate::modules::tasks::presentation::components::task_card::{TaskCardBusy, TaskCardHandlers};
use crate::modules::tasks::presentation::components::task_node::TaskNode;
use crate::modules::tasks::presentation::state::board_state::BoardState;
use crate::modules::tasks::presentation::state::graph_geometry::{
    Point, Viewport, edge_path, input_anchor, output_anchor,
};
use crate::modules::tasks::presentation::state::graph_interaction::DragMode;
use crate::modules::tasks::presentation::state::graph_model::{
    EdgeView, GraphView, NewTaskLink, NodeView, node_key,
};

/// Distancia entre puntos de la rejilla de fondo, en coordenadas de lienzo.
const GRID_SIZE: f64 = 24.0;

const ZOOM_IN: Msg = Msg {
    en: "Zoom in",
    es: "Acercar",
    zh: "放大",
    de: "Vergrößern",
    fr: "Zoom avant",
};

const ZOOM_OUT: Msg = Msg {
    en: "Zoom out",
    es: "Alejar",
    zh: "缩小",
    de: "Verkleinern",
    fr: "Zoom arrière",
};

/// Estilo del fondo de puntos: sigue el desplazamiento y el zoom de la vista.
pub fn grid_style(viewport: &Viewport) -> String {
    let size: f64 = GRID_SIZE * viewport.zoom;
    format!(
        "background-size: {size:.2}px {size:.2}px; background-position: {:.2}px {:.2}px;",
        viewport.x, viewport.y
    )
}

/// Tamaño (ancho, alto) en píxeles del lienzo montado, si ya existe.
pub fn canvas_size(canvas: NodeRef<html::Div>) -> Option<(f64, f64)> {
    canvas
        .get_untracked()
        .map(|el| (f64::from(el.client_width()), f64::from(el.client_height())))
}

/// Punto de pantalla (client) relativo a la esquina del lienzo. Solo en el navegador se
/// conoce el rectángulo del lienzo; fuera de él (SSR, sin eventos) se devuelve tal cual.
fn canvas_point(canvas: NodeRef<html::Div>, client_x: f64, client_y: f64) -> Point {
    #[cfg(feature = "hydrate")]
    if let Some(el) = canvas.get_untracked() {
        let rect = el.get_bounding_client_rect();
        return Point::new(client_x - rect.left(), client_y - rect.top());
    }
    #[cfg(not(feature = "hydrate"))]
    let _canvas: NodeRef<html::Div> = canvas;
    Point::new(client_x, client_y)
}

/// Centro del lienzo, para los botones de zoom.
fn canvas_center(canvas: NodeRef<html::Div>) -> Point {
    let (width, height): (f64, f64) = canvas_size(canvas).unwrap_or_default();
    Point::new(width / 2.0, height / 2.0)
}

/// Lienzo con zoom y desplazamiento: aristas SVG y nodos HTML posicionados en coordenadas de
/// lienzo. Los gestos (arrastrar fondo, nodo o puerto) los resuelve `BoardState`.
#[component]
pub fn TaskGraphCanvas(
    state: BoardState,
    canvas: NodeRef<html::Div>,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
) -> impl IntoView {
    // Captura el puntero en el lienzo: los movimientos y el soltar llegan aunque el cursor
    // salga del nodo o del propio lienzo.
    let capture = move |ev: &PointerEvent| {
        if let Some(el) = canvas.get_untracked() {
            el.set_pointer_capture(ev.pointer_id()).ok();
        }
    };
    let point_of = move |ev: &PointerEvent| {
        canvas_point(canvas, f64::from(ev.client_x()), f64::from(ev.client_y()))
    };

    let on_background_down = move |ev: PointerEvent| {
        if ev.button() != 0 {
            return;
        }
        capture(&ev);
        state.pointer_down_background(point_of(&ev));
    };
    let on_node_down = move |id: String, ev: PointerEvent| {
        if ev.button() != 0 {
            return;
        }
        ev.stop_propagation();
        capture(&ev);
        state.pointer_down_node(id, point_of(&ev));
    };
    let on_port_down = move |id: String, ev: PointerEvent| {
        if ev.button() != 0 {
            return;
        }
        capture(&ev);
        state.pointer_down_port(id, point_of(&ev));
    };
    let on_wheel = move |ev: WheelEvent| {
        ev.prevent_default();
        let point: Point = canvas_point(canvas, f64::from(ev.client_x()), f64::from(ev.client_y()));
        state.wheel(point, ev.delta_y());
    };

    let nodes: Signal<Vec<NodeView>> =
        Signal::derive(move || state.view.with(|g: &GraphView| g.nodes.clone()));
    let edges: Signal<Vec<EdgeView>> =
        Signal::derive(move || state.view.with(|g: &GraphView| g.edges.clone()));
    let zoom: Signal<f64> = Signal::derive(move || state.viewport.get().zoom);
    // Arista en construcción: de la salida del origen al cursor (o al nodo sobre el que está).
    let preview: Memo<Option<String>> = Memo::new(move |_| {
        let DragMode::Connect {
            from,
            cursor,
            target,
        } = state.drag.get()
        else {
            return None;
        };
        state.positions.with(|positions| {
            let start: Point = output_anchor(*positions.get(&from)?);
            let end: Point = target
                .as_ref()
                .and_then(|id: &String| positions.get(id))
                .map_or(cursor, |p: &Point| input_anchor(*p));
            Some(edge_path(start, end))
        })
    });
    let canvas_class: Memo<&'static str> = Memo::new(move |_| {
        if matches!(state.drag.get(), DragMode::Idle) {
            "graph-canvas"
        } else {
            "graph-canvas graph-canvas-busy"
        }
    });

    view! {
        <div
            node_ref=canvas
            class=move || canvas_class.get()
            style=move || state.viewport.with(grid_style)
            on:pointerdown=on_background_down
            on:pointermove=move |ev: PointerEvent| state.pointer_move(point_of(&ev))
            on:pointerup=move |ev: PointerEvent| state.pointer_up(point_of(&ev))
            on:pointercancel=move |_| state.pointer_cancel()
            on:wheel=on_wheel
        >
            <div
                class="graph-layer"
                style=move || format!("transform: {};", state.viewport.get().transform_css())
            >
                <svg class="graph-edges" width="1" height="1" aria-hidden="true">
                    <GraphArrowDefs />
                    <For
                        each=move || edges.get()
                        key=|e: &EdgeView| format!("{}|{:?}|{:?}", e.key(), e.state, e.label)
                        children=move |edge: EdgeView| {
                            let from_id: String = edge.predecessor_id.clone();
                            let to_id: String = edge.successor_id.clone();
                            let from: Signal<Point> = Signal::derive(move || {
                                state
                                    .positions
                                    .with(|p| p.get(&from_id).copied().map(output_anchor))
                                    .unwrap_or_default()
                            });
                            let to: Signal<Point> = Signal::derive(move || {
                                state
                                    .positions
                                    .with(|p| p.get(&to_id).copied().map(input_anchor))
                                    .unwrap_or_default()
                            });
                            let pair: (String, String) = (
                                edge.predecessor_id.clone(),
                                edge.successor_id.clone(),
                            );
                            let selected_pair: (String, String) = pair.clone();
                            let selected: Signal<bool> = Signal::derive(move || {
                                state
                                    .selected_edge
                                    .with(|s| s.as_ref() == Some(&selected_pair))
                            });
                            view! {
                                <GraphEdgeLine
                                    edge
                                    from
                                    to
                                    selected
                                    on_select=Callback::new(move |_| {
                                        state.select_edge(pair.0.clone(), pair.1.clone());
                                    })
                                    on_delete=Callback::new(move |_| state.delete_selected_edge())
                                />
                            }
                        }
                    />
                    {move || {
                        preview
                            .get()
                            .map(|d: String| view! { <path class="graph-edge-preview" d=d /> })
                    }}
                </svg>
                <For
                    each=move || nodes.get()
                    key=node_key
                    children=move |node: NodeView| {
                        let id: String = node.task.id.clone();
                        let position: Signal<Point> = {
                            let id: String = id.clone();
                            Signal::derive(move || {
                                state
                                    .positions
                                    .with(|p| p.get(&id).copied())
                                    .unwrap_or_default()
                            })
                        };
                        let selected: Signal<bool> = {
                            let id: String = id.clone();
                            Signal::derive(move || {
                                state.selected_node.with(|s| s.as_deref() == Some(id.as_str()))
                            })
                        };
                        let connect_target: Signal<bool> = {
                            let id: String = id.clone();
                            Signal::derive(move || {
                                state.drag.with(|d: &DragMode| {
                                    matches!(
                                        d,
                                        DragMode::Connect { target: Some(t), .. } if *t == id
                                    )
                                })
                            })
                        };
                        let drag_id: String = id.clone();
                        let port_id: String = id.clone();
                        view! {
                            <TaskNode
                                node
                                position
                                selected
                                connect_target
                                handlers
                                busy
                                on_drag_start=Callback::new(move |ev: PointerEvent| {
                                    on_node_down(drag_id.clone(), ev);
                                })
                                on_connect_start=Callback::new(move |ev: PointerEvent| {
                                    on_port_down(port_id.clone(), ev);
                                })
                                on_add_next=Callback::new(move |anchor: String| {
                                    state.open_create_linked(NewTaskLink::Next(anchor));
                                })
                                on_add_parallel=Callback::new(move |anchor: String| {
                                    state.open_create_linked(NewTaskLink::Parallel(anchor));
                                })
                            />
                        }
                    }
                />
            </div>
            <ZoomControls
                zoom
                zoom_in_label=ZOOM_IN.get()
                zoom_out_label=ZOOM_OUT.get()
                on_zoom_in=Callback::new(move |_| state.zoom_step(true, canvas_center(canvas)))
                on_zoom_out=Callback::new(move |_| state.zoom_step(false, canvas_center(canvas)))
            />
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn grid_follows_pan_and_zoom() {
        let viewport: Viewport = Viewport {
            x: 10.0,
            y: -4.5,
            zoom: 0.5,
        };
        assert_eq!(
            grid_style(&viewport),
            "background-size: 12.00px 12.00px; background-position: 10.00px -4.50px;"
        );
    }
}
