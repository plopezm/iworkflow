//! Lógica pura de arrastrar una tarea suelta del panel izquierdo al lienzo: prueba de impacto
//! sobre el rectángulo del lienzo y posición de caída. Sin Leptos ni DOM, para poder probarla.

use crate::modules::tasks::presentation::state::graph_geometry::{
    NODE_HEIGHT, NODE_WIDTH, Point, Rect, Viewport,
};

/// Arrastre en curso de una tarea suelta; `pointer` está en coordenadas de ventana (client).
#[derive(Debug, Clone, PartialEq)]
pub struct LooseDrag {
    pub task_id: String,
    pub pointer: Point,
}

/// Ancho de ventana (px) por debajo del cual el tablero usa subpestañas en lugar de dos paneles.
pub const NARROW_BREAKPOINT: f64 = 768.0;

/// Indica si una ventana de `width` px usa el diseño estrecho (subpestañas).
pub fn is_narrow_viewport(width: f64) -> bool {
    width < NARROW_BREAKPOINT
}

/// Posición de nodo (esquina superior izquierda) que deja su centro en `centre`.
fn node_position_centred_at(centre: Point) -> Point {
    Point::new(centre.x - NODE_WIDTH / 2.0, centre.y - NODE_HEIGHT / 2.0)
}

/// Indica si el puntero (coordenadas de ventana) está sobre el rectángulo del lienzo.
pub fn is_over_canvas(client: Point, canvas: &Rect) -> bool {
    canvas.width > 0.0 && canvas.height > 0.0 && canvas.contains(client, 0.0)
}

/// Posición del nodo al soltar sobre el lienzo, con el nodo centrado bajo el puntero. `None`
/// si el puntero no está sobre el lienzo (la tarea sigue suelta).
pub fn drop_position(client: Point, canvas: &Rect, viewport: &Viewport) -> Option<Point> {
    if !is_over_canvas(client, canvas) {
        return None;
    }
    let local: Point = Point::new(client.x - canvas.x, client.y - canvas.y);
    Some(node_position_centred_at(viewport.screen_to_canvas(local)))
}

/// Posición del nodo cuando se añade con el botón: centrado en la parte visible del lienzo
/// de tamaño `size` (ancho, alto).
pub fn centre_position(size: (f64, f64), viewport: &Viewport) -> Point {
    let centre: Point = viewport.screen_to_canvas(Point::new(size.0 / 2.0, size.1 / 2.0));
    node_position_centred_at(centre)
}

#[cfg(test)]
mod tests {
    use super::*;

    const CANVAS: Rect = Rect {
        x: 300.0,
        y: 50.0,
        width: 800.0,
        height: 600.0,
    };

    #[test]
    fn only_points_inside_the_canvas_hit_it() {
        assert!(is_over_canvas(Point::new(300.0, 50.0), &CANVAS));
        assert!(is_over_canvas(Point::new(1100.0, 650.0), &CANVAS));
        // El panel está a la izquierda: soltar encima no añade nada.
        assert!(!is_over_canvas(Point::new(120.0, 200.0), &CANVAS));
        assert!(!is_over_canvas(Point::new(500.0, 20.0), &CANVAS));
        assert!(!is_over_canvas(Point::new(1101.0, 300.0), &CANVAS));
    }

    #[test]
    fn an_empty_canvas_rect_is_never_hit() {
        let empty: Rect = Rect {
            x: 10.0,
            y: 10.0,
            width: 0.0,
            height: 0.0,
        };
        assert!(!is_over_canvas(Point::new(10.0, 10.0), &empty));
    }

    #[test]
    fn narrow_layout_starts_below_the_breakpoint() {
        assert!(is_narrow_viewport(767.0));
        assert!(!is_narrow_viewport(768.0));
    }

    #[test]
    fn drop_centres_the_node_under_the_pointer_in_canvas_coordinates() {
        let viewport: Viewport = Viewport::default();
        // Puntero en (700, 350) → local (400, 300) → esquina (400-132, 300-70).
        assert_eq!(
            drop_position(Point::new(700.0, 350.0), &CANVAS, &viewport),
            Some(Point::new(268.0, 230.0))
        );
    }

    #[test]
    fn drop_accounts_for_pan_and_zoom() {
        let viewport: Viewport = Viewport {
            x: 100.0,
            y: -50.0,
            zoom: 2.0,
        };
        // Local (400, 300) → lienzo ((400-100)/2, (300+50)/2) = (150, 175).
        assert_eq!(
            drop_position(Point::new(700.0, 350.0), &CANVAS, &viewport),
            Some(Point::new(150.0 - 132.0, 175.0 - 70.0))
        );
    }

    #[test]
    fn drop_outside_the_canvas_yields_nothing() {
        assert_eq!(
            drop_position(Point::new(100.0, 100.0), &CANVAS, &Viewport::default()),
            None
        );
    }

    #[test]
    fn button_places_the_node_at_the_visible_centre() {
        assert_eq!(
            centre_position((800.0, 600.0), &Viewport::default()),
            Point::new(268.0, 230.0)
        );
        let panned: Viewport = Viewport {
            x: -200.0,
            y: 0.0,
            zoom: 1.0,
        };
        assert_eq!(
            centre_position((800.0, 600.0), &panned),
            Point::new(468.0, 230.0)
        );
    }
}
