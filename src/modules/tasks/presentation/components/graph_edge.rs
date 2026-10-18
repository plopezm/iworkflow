use leptos::ev::{MouseEvent, PointerEvent};
use leptos::prelude::*;

use crate::core::i18n::Msg;
use crate::modules::tasks::presentation::state::graph_geometry::{Point, edge_midpoint, edge_path};
use crate::modules::tasks::presentation::state::graph_model::{EdgeState, EdgeView};

/// Radio del botón de borrado de una arista seleccionada.
const DELETE_RADIUS: f64 = 12.0;

const REMOVE_DEPENDENCY: Msg = Msg {
    en: "Remove dependency",
    es: "Quitar dependencia",
    zh: "移除依赖",
    de: "Abhängigkeit entfernen",
    fr: "Supprimer la dépendance",
};

/// Nombres base de los `<marker>` de punta de flecha, uno por estado. Los ids de SVG son
/// globales al documento, así que el id real lleva además el ámbito del lienzo (`marker_id`).
pub const ARROW_DONE: &str = "iwf-arrow-done";
pub const ARROW_WORKING: &str = "iwf-arrow-working";
pub const ARROW_PENDING: &str = "iwf-arrow-pending";
pub const ARROW_ACTIVE: &str = "iwf-arrow-active";

/// Clases CSS de una arista según su estado y si está seleccionada.
pub fn edge_class(state: EdgeState, selected: bool) -> String {
    let variant: &str = match state {
        EdgeState::Done => "graph-edge-done",
        EdgeState::Working => "graph-edge-working",
        EdgeState::Pending => "graph-edge-pending",
    };
    if selected {
        format!("graph-edge {variant} graph-edge-selected")
    } else {
        format!("graph-edge {variant}")
    }
}

/// Ámbito de ids de un lienzo: distinto por pestaña (su ruta), para que varios tableros en el
/// mismo documento no compartan `<marker>`. Determinista, así que coincide entre SSR e
/// hidratación; los caracteres que no son alfanuméricos se escapan de forma inyectiva.
pub fn marker_scope(tab_path: Option<&str>) -> String {
    let Some(path) = tab_path else {
        return "board".to_string();
    };
    path.chars()
        .map(|c: char| {
            if c.is_ascii_alphanumeric() {
                c.to_string()
            } else {
                format!("_{:x}_", u32::from(c))
            }
        })
        .collect()
}

/// Id del `<marker>` de nombre base `name` dentro del ámbito `scope`.
pub fn marker_id(scope: &str, name: &str) -> String {
    format!("{name}-{scope}")
}

/// Valor del atributo `marker-end` de la arista.
pub fn arrow_marker(state: EdgeState, selected: bool, scope: &str) -> String {
    let name: &str = if selected {
        ARROW_ACTIVE
    } else {
        match state {
            EdgeState::Done => ARROW_DONE,
            EdgeState::Working => ARROW_WORKING,
            EdgeState::Pending => ARROW_PENDING,
        }
    };
    format!("url(#{})", marker_id(scope, name))
}

/// Definiciones SVG de las puntas de flecha (una por estado más la de selección).
#[component]
pub fn GraphArrowDefs(#[prop(into)] scope: String) -> impl IntoView {
    view! {
        <defs>
            {[
                (ARROW_DONE, "graph-arrow-done"),
                (ARROW_WORKING, "graph-arrow-working"),
                (ARROW_PENDING, "graph-arrow-pending"),
                (ARROW_ACTIVE, "graph-arrow-active"),
            ]
                .into_iter()
                .map(|(name, class): (&str, &str)| {
                    view! {
                        <marker
                            id=marker_id(&scope, name)
                            class=class
                            viewBox="0 0 10 10"
                            refX="9"
                            refY="5"
                            markerWidth="12"
                            markerHeight="12"
                            markerUnits="userSpaceOnUse"
                            orient="auto"
                        >
                            <path d="M 0 1 L 10 5 L 0 9 z" />
                        </marker>
                    }
                })
                .collect_view()}
        </defs>
    }
}

/// Arista dirigida entre el puerto de salida de la predecesora y el de entrada de la sucesora.
/// Al seleccionarla muestra un botón para borrarla.
#[component]
pub fn GraphEdgeLine(
    edge: EdgeView,
    #[prop(into)] marker_scope: String,
    #[prop(into)] from: Signal<Point>,
    #[prop(into)] to: Signal<Point>,
    #[prop(into)] selected: Signal<bool>,
    on_select: Callback<()>,
    on_delete: Callback<()>,
) -> impl IntoView {
    let state: EdgeState = edge.state;
    let path: Memo<String> = Memo::new(move |_| edge_path(from.get(), to.get()));
    let middle: Memo<Point> = Memo::new(move |_| edge_midpoint(from.get(), to.get()));
    let label: Option<String> = edge.label.clone();

    view! {
        <g class=move || edge_class(state, selected.get())>
            // Zona de clic ancha e invisible: la línea visible es demasiado fina para tocarla.
            <path
                class="graph-edge-hit"
                d=move || path.get()
                on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
                on:click=move |ev: MouseEvent| {
                    ev.stop_propagation();
                    on_select.run(());
                }
            />
            <path
                class="graph-edge-line"
                d=move || path.get()
                marker-end=move || arrow_marker(state, selected.get(), &marker_scope)
            />
            {label
                .map(|text: String| {
                    view! {
                        <text
                            class="graph-edge-label"
                            x=move || middle.get().x
                            y=move || middle.get().y - 10.0
                            text-anchor="middle"
                        >
                            {text}
                        </text>
                    }
                })}
            <Show when=move || selected.get()>
                <g
                    class="graph-edge-delete"
                    transform=move || {
                        let point: Point = middle.get();
                        format!("translate({:.2} {:.2})", point.x, point.y)
                    }
                    role="button"
                    aria-label=REMOVE_DEPENDENCY.get()
                    on:pointerdown=|ev: PointerEvent| ev.stop_propagation()
                    on:click=move |ev: MouseEvent| {
                        ev.stop_propagation();
                        on_delete.run(());
                    }
                >
                    <title>{REMOVE_DEPENDENCY.get()}</title>
                    <circle r=DELETE_RADIUS />
                    <path d="M -4 -4 L 4 4 M 4 -4 L -4 4" />
                </g>
            </Show>
        </g>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn edge_class_follows_state_and_selection() {
        assert_eq!(
            edge_class(EdgeState::Done, false),
            "graph-edge graph-edge-done"
        );
        assert_eq!(
            edge_class(EdgeState::Working, true),
            "graph-edge graph-edge-working graph-edge-selected"
        );
        assert!(edge_class(EdgeState::Pending, false).ends_with("graph-edge-pending"));
    }

    #[test]
    fn arrow_marker_points_to_the_state_marker_or_the_active_one() {
        assert_eq!(
            arrow_marker(EdgeState::Done, false, "s"),
            "url(#iwf-arrow-done-s)"
        );
        assert_eq!(
            arrow_marker(EdgeState::Pending, false, "s"),
            "url(#iwf-arrow-pending-s)"
        );
        assert_eq!(
            arrow_marker(EdgeState::Working, true, "s"),
            "url(#iwf-arrow-active-s)"
        );
    }

    #[test]
    fn marker_ids_differ_between_canvases_and_match_their_references() {
        let first: String = marker_scope(Some("/board/w1"));
        let second: String = marker_scope(Some("/board/w2"));
        assert_ne!(
            marker_id(&first, ARROW_DONE),
            marker_id(&second, ARROW_DONE)
        );
        assert_eq!(
            arrow_marker(EdgeState::Done, false, &first),
            format!("url(#{})", marker_id(&first, ARROW_DONE))
        );
    }

    #[test]
    fn marker_scope_is_a_valid_injective_id_fragment() {
        assert_eq!(marker_scope(None), "board");
        let scope: String = marker_scope(Some("/board/a-b"));
        assert!(
            scope
                .chars()
                .all(|c: char| c.is_ascii_alphanumeric() || c == '_')
        );
        assert_ne!(marker_scope(Some("/a/b")), marker_scope(Some("/a-b")));
        assert_ne!(marker_scope(Some("/a_b")), marker_scope(Some("/a/b")));
    }
}
