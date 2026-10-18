//! Geometría pura del lienzo del tablero: transformaciones pantalla ↔ lienzo, zoom, encuadre,
//! puntos de anclaje y trazado de las aristas. Sin Leptos ni DOM, para poder probarla.

/// Ancho de un nodo en coordenadas de lienzo.
pub const NODE_WIDTH: f64 = 264.0;
/// Alto de un nodo en coordenadas de lienzo.
pub const NODE_HEIGHT: f64 = 140.0;
/// Zoom mínimo permitido.
pub const MIN_ZOOM: f64 = 0.25;
/// Zoom máximo permitido.
pub const MAX_ZOOM: f64 = 2.0;
/// Margen (en píxeles de pantalla) que deja «ajustar vista» alrededor del grafo.
pub const FIT_PADDING: f64 = 56.0;
/// Paso de los botones de zoom.
pub const ZOOM_STEP: f64 = 1.2;
/// Margen extra (coordenadas de lienzo) alrededor de un nodo al soltar una arista sobre él,
/// para que también valga soltar sobre su puerto de entrada.
pub const DROP_MARGIN: f64 = 14.0;
/// Sensibilidad de la rueda del ratón.
const WHEEL_SENSITIVITY: f64 = 0.0015;
/// Desplazamiento máximo de rueda que se tiene en cuenta en un solo evento.
const WHEEL_MAX_DELTA: f64 = 400.0;
/// Separación mínima entre un extremo de arista y sus puntos de control.
const MIN_CONTROL_OFFSET: f64 = 48.0;

/// Punto 2D.
#[derive(Debug, Clone, Copy, PartialEq, Default)]
pub struct Point {
    pub x: f64,
    pub y: f64,
}

impl Point {
    pub const fn new(x: f64, y: f64) -> Self {
        Self { x, y }
    }
}

/// Rectángulo alineado con los ejes.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Rect {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

impl Rect {
    /// Rectángulo de un nodo cuya esquina superior izquierda es `position`.
    pub fn of_node(position: Point) -> Self {
        Self {
            x: position.x,
            y: position.y,
            width: NODE_WIDTH,
            height: NODE_HEIGHT,
        }
    }

    /// Indica si `point` cae dentro del rectángulo ampliado `margin` por cada lado.
    pub fn contains(&self, point: Point, margin: f64) -> bool {
        point.x >= self.x - margin
            && point.x <= self.x + self.width + margin
            && point.y >= self.y - margin
            && point.y <= self.y + self.height + margin
    }

    /// Indica si dos rectángulos se solapan (compartir solo un borde no cuenta).
    pub fn intersects(&self, other: &Rect) -> bool {
        self.x < other.x + other.width
            && other.x < self.x + self.width
            && self.y < other.y + other.height
            && other.y < self.y + self.height
    }
}

/// Vista del lienzo: `pantalla = lienzo * zoom + (x, y)`.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Viewport {
    pub x: f64,
    pub y: f64,
    pub zoom: f64,
}

impl Default for Viewport {
    fn default() -> Self {
        Self {
            x: 0.0,
            y: 0.0,
            zoom: 1.0,
        }
    }
}

/// Limita un zoom al rango permitido.
pub fn clamp_zoom(zoom: f64) -> f64 {
    zoom.clamp(MIN_ZOOM, MAX_ZOOM)
}

impl Viewport {
    /// Convierte un punto de pantalla (relativo al lienzo) a coordenadas de lienzo.
    pub fn screen_to_canvas(&self, screen: Point) -> Point {
        Point::new(
            (screen.x - self.x) / self.zoom,
            (screen.y - self.y) / self.zoom,
        )
    }

    /// Convierte un punto de lienzo a pantalla.
    pub fn canvas_to_screen(&self, canvas: Point) -> Point {
        Point::new(canvas.x * self.zoom + self.x, canvas.y * self.zoom + self.y)
    }

    /// Vista desplazada `dx`, `dy` píxeles de pantalla.
    pub fn panned(&self, dx: f64, dy: f64) -> Self {
        Self {
            x: self.x + dx,
            y: self.y + dy,
            zoom: self.zoom,
        }
    }

    /// Vista con el zoom multiplicado por `factor`, manteniendo fijo el punto de pantalla
    /// `anchor` (el cursor).
    pub fn zoomed_at(&self, anchor: Point, factor: f64) -> Self {
        let zoom: f64 = clamp_zoom(self.zoom * factor);
        let canvas: Point = self.screen_to_canvas(anchor);
        Self {
            x: anchor.x - canvas.x * zoom,
            y: anchor.y - canvas.y * zoom,
            zoom,
        }
    }

    /// Valor de la propiedad CSS `transform` de la capa de contenido.
    pub fn transform_css(&self) -> String {
        format!(
            "translate({:.2}px, {:.2}px) scale({:.4})",
            self.x, self.y, self.zoom
        )
    }
}

/// Factor de zoom que corresponde a un desplazamiento de rueda (`delta_y > 0` aleja).
pub fn wheel_zoom_factor(delta_y: f64) -> f64 {
    let delta: f64 = delta_y.clamp(-WHEEL_MAX_DELTA, WHEEL_MAX_DELTA);
    (-delta * WHEEL_SENSITIVITY).exp()
}

/// Rectángulo mínimo que contiene a todos los nodos situados en `positions`.
pub fn bounds_of_nodes(positions: &[Point]) -> Option<Rect> {
    let first: Point = *positions.first()?;
    let (min_x, min_y, max_x, max_y): (f64, f64, f64, f64) = positions.iter().fold(
        (first.x, first.y, first.x, first.y),
        |(min_x, min_y, max_x, max_y): (f64, f64, f64, f64), p: &Point| {
            (
                min_x.min(p.x),
                min_y.min(p.y),
                max_x.max(p.x),
                max_y.max(p.y),
            )
        },
    );
    Some(Rect {
        x: min_x,
        y: min_y,
        width: max_x - min_x + NODE_WIDTH,
        height: max_y - min_y + NODE_HEIGHT,
    })
}

/// Vista que encuadra `bounds` dentro de un lienzo de `size` (ancho, alto) en píxeles.
/// Sin contenido vuelve a la vista por defecto. No amplía por encima de zoom 1.
pub fn fit_viewport(bounds: Option<Rect>, size: (f64, f64)) -> Viewport {
    let (width, height): (f64, f64) = size;
    let Some(bounds) = bounds else {
        return Viewport::default();
    };
    if width <= 0.0 || height <= 0.0 || bounds.width <= 0.0 || bounds.height <= 0.0 {
        return Viewport::default();
    }
    let available_w: f64 = (width - 2.0 * FIT_PADDING).max(1.0);
    let available_h: f64 = (height - 2.0 * FIT_PADDING).max(1.0);
    let zoom: f64 =
        clamp_zoom((available_w / bounds.width).min(available_h / bounds.height)).min(1.0);
    Viewport {
        x: (width - bounds.width * zoom) / 2.0 - bounds.x * zoom,
        y: (height - bounds.height * zoom) / 2.0 - bounds.y * zoom,
        zoom,
    }
}

/// Punto de salida (puerto derecho) de un nodo.
pub fn output_anchor(position: Point) -> Point {
    Point::new(position.x + NODE_WIDTH, position.y + NODE_HEIGHT / 2.0)
}

/// Punto de entrada (puerto izquierdo) de un nodo.
pub fn input_anchor(position: Point) -> Point {
    Point::new(position.x, position.y + NODE_HEIGHT / 2.0)
}

/// Separación de los puntos de control de una Bézier entre `from` y `to`: proporcional a la
/// distancia horizontal y mayor cuando la arista vuelve hacia atrás.
fn control_offset(from: Point, to: Point) -> f64 {
    let dx: f64 = to.x - from.x;
    if dx >= 0.0 {
        (dx * 0.5).max(MIN_CONTROL_OFFSET)
    } else {
        MIN_CONTROL_OFFSET + dx.abs() * 0.5
    }
}

/// Puntos de control de la Bézier cúbica de una arista.
fn control_points(from: Point, to: Point) -> (Point, Point) {
    let offset: f64 = control_offset(from, to);
    (
        Point::new(from.x + offset, from.y),
        Point::new(to.x - offset, to.y),
    )
}

/// Atributo `d` de un `<path>` SVG: Bézier cúbica suave de `from` a `to`.
pub fn edge_path(from: Point, to: Point) -> String {
    let (c1, c2): (Point, Point) = control_points(from, to);
    format!(
        "M {:.2} {:.2} C {:.2} {:.2}, {:.2} {:.2}, {:.2} {:.2}",
        from.x, from.y, c1.x, c1.y, c2.x, c2.y, to.x, to.y
    )
}

/// Punto central de la curva (t = 0,5): ancla de la etiqueta y del botón de borrado.
pub fn edge_midpoint(from: Point, to: Point) -> Point {
    let (c1, c2): (Point, Point) = control_points(from, to);
    Point::new(
        (from.x + 3.0 * c1.x + 3.0 * c2.x + to.x) / 8.0,
        (from.y + 3.0 * c1.y + 3.0 * c2.y + to.y) / 8.0,
    )
}

/// Id del nodo bajo `point` (el último de la lista, que es el que se pinta encima).
pub fn node_at(point: Point, nodes: &[(String, Point)]) -> Option<String> {
    nodes
        .iter()
        .rev()
        .find(|(_, position): &&(String, Point)| {
            Rect::of_node(*position).contains(point, DROP_MARGIN)
        })
        .map(|(id, _): &(String, Point)| id.clone())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn approx(a: f64, b: f64) -> bool {
        (a - b).abs() < 1e-9
    }

    #[test]
    fn screen_and_canvas_transforms_are_inverse() {
        let viewport: Viewport = Viewport {
            x: 120.0,
            y: -40.0,
            zoom: 0.5,
        };
        let canvas: Point = Point::new(300.0, 80.0);
        let screen: Point = viewport.canvas_to_screen(canvas);
        assert_eq!(screen, Point::new(270.0, 0.0));
        let back: Point = viewport.screen_to_canvas(screen);
        assert!(approx(back.x, canvas.x) && approx(back.y, canvas.y));
    }

    #[test]
    fn panning_moves_the_view_without_touching_zoom() {
        let viewport: Viewport = Viewport::default().panned(15.0, -5.0);
        assert_eq!((viewport.x, viewport.y, viewport.zoom), (15.0, -5.0, 1.0));
    }

    #[test]
    fn zooming_keeps_the_point_under_the_cursor_fixed() {
        let viewport: Viewport = Viewport {
            x: 30.0,
            y: 10.0,
            zoom: 1.0,
        };
        let anchor: Point = Point::new(200.0, 150.0);
        let before: Point = viewport.screen_to_canvas(anchor);
        let zoomed: Viewport = viewport.zoomed_at(anchor, 1.5);
        let after: Point = zoomed.screen_to_canvas(anchor);
        assert!(approx(zoomed.zoom, 1.5));
        assert!(approx(before.x, after.x) && approx(before.y, after.y));
    }

    #[test]
    fn zoom_is_clamped_to_its_range() {
        let viewport: Viewport = Viewport::default();
        assert!(approx(
            viewport.zoomed_at(Point::default(), 100.0).zoom,
            MAX_ZOOM
        ));
        assert!(approx(
            viewport.zoomed_at(Point::default(), 0.0001).zoom,
            MIN_ZOOM
        ));
    }

    #[test]
    fn wheel_down_zooms_out_and_up_zooms_in() {
        assert!(wheel_zoom_factor(100.0) < 1.0);
        assert!(wheel_zoom_factor(-100.0) > 1.0);
        assert!(approx(wheel_zoom_factor(0.0), 1.0));
        // Un único evento enorme no dispara el zoom.
        assert!(approx(
            wheel_zoom_factor(1_000_000.0),
            wheel_zoom_factor(WHEEL_MAX_DELTA)
        ));
    }

    #[test]
    fn bounds_cover_every_node() {
        assert_eq!(bounds_of_nodes(&[]), None);
        let bounds: Rect =
            bounds_of_nodes(&[Point::new(10.0, 20.0), Point::new(500.0, -30.0)]).expect("bounds");
        assert_eq!(bounds.x, 10.0);
        assert_eq!(bounds.y, -30.0);
        assert_eq!(bounds.width, 490.0 + NODE_WIDTH);
        assert_eq!(bounds.height, 50.0 + NODE_HEIGHT);
    }

    #[test]
    fn fit_centers_the_content_and_never_zooms_in_past_one() {
        let bounds: Rect = Rect::of_node(Point::new(0.0, 0.0));
        let viewport: Viewport = fit_viewport(Some(bounds), (2000.0, 1200.0));
        assert!(approx(viewport.zoom, 1.0));
        let center: Point =
            viewport.canvas_to_screen(Point::new(bounds.width / 2.0, bounds.height / 2.0));
        assert!(approx(center.x, 1000.0) && approx(center.y, 600.0));
    }

    #[test]
    fn fit_shrinks_large_graphs_and_resets_when_empty() {
        let bounds: Rect = Rect {
            x: 0.0,
            y: 0.0,
            width: 4000.0,
            height: 1000.0,
        };
        let viewport: Viewport = fit_viewport(Some(bounds), (800.0, 600.0));
        // 688 / 4000 queda por debajo del mínimo: se limita a MIN_ZOOM.
        assert!(approx(viewport.zoom, MIN_ZOOM));
        let medium: Rect = Rect {
            x: 0.0,
            y: 0.0,
            width: 1488.0,
            height: 300.0,
        };
        assert!(approx(
            fit_viewport(Some(medium), (800.0, 600.0)).zoom,
            0.4623655913978495
        ));
        assert_eq!(fit_viewport(None, (800.0, 600.0)), Viewport::default());
        assert_eq!(fit_viewport(Some(bounds), (0.0, 0.0)), Viewport::default());
    }

    #[test]
    fn anchors_sit_on_the_vertical_middle_of_each_side() {
        let position: Point = Point::new(100.0, 50.0);
        assert_eq!(
            output_anchor(position),
            Point::new(100.0 + NODE_WIDTH, 50.0 + NODE_HEIGHT / 2.0)
        );
        assert_eq!(
            input_anchor(position),
            Point::new(100.0, 50.0 + NODE_HEIGHT / 2.0)
        );
    }

    #[test]
    fn edge_path_is_a_cubic_curve_between_the_anchors() {
        let path: String = edge_path(Point::new(0.0, 0.0), Point::new(400.0, 100.0));
        assert_eq!(
            path,
            "M 0.00 0.00 C 200.00 0.00, 200.00 100.00, 400.00 100.00"
        );
    }

    #[test]
    fn backwards_edges_bulge_further_than_the_minimum() {
        let forward: String = edge_path(Point::new(0.0, 0.0), Point::new(10.0, 0.0));
        let backward: String = edge_path(Point::new(300.0, 0.0), Point::new(0.0, 0.0));
        assert!(forward.contains("C 48.00 0.00"));
        assert!(backward.contains("C 498.00 0.00"));
    }

    #[test]
    fn midpoint_lies_halfway_on_a_symmetric_curve() {
        let mid: Point = edge_midpoint(Point::new(0.0, 0.0), Point::new(400.0, 100.0));
        assert!(approx(mid.x, 200.0) && approx(mid.y, 50.0));
    }

    #[test]
    fn rects_contain_with_margin_and_intersect_without_touching() {
        let rect: Rect = Rect::of_node(Point::new(0.0, 0.0));
        assert!(rect.contains(Point::new(10.0, 10.0), 0.0));
        assert!(!rect.contains(Point::new(-5.0, 10.0), 0.0));
        assert!(rect.contains(Point::new(-5.0, 10.0), DROP_MARGIN));
        let touching: Rect = Rect::of_node(Point::new(NODE_WIDTH, 0.0));
        assert!(!rect.intersects(&touching));
        let overlapping: Rect = Rect::of_node(Point::new(NODE_WIDTH - 1.0, 0.0));
        assert!(rect.intersects(&overlapping));
    }

    #[test]
    fn node_at_prefers_the_topmost_node_and_accepts_the_input_port() {
        let nodes: Vec<(String, Point)> = vec![
            ("a".to_string(), Point::new(0.0, 0.0)),
            ("b".to_string(), Point::new(100.0, 20.0)),
        ];
        assert_eq!(
            node_at(Point::new(120.0, 30.0), &nodes).as_deref(),
            Some("b")
        );
        assert_eq!(
            node_at(Point::new(10.0, 10.0), &nodes).as_deref(),
            Some("a")
        );
        // Sobre el puerto de entrada (ligeramente fuera del borde izquierdo).
        assert_eq!(
            node_at(Point::new(-6.0, 70.0), &nodes).as_deref(),
            Some("a")
        );
        assert_eq!(node_at(Point::new(-500.0, -500.0), &nodes), None);
    }

    #[test]
    fn transform_css_scales_and_translates() {
        let viewport: Viewport = Viewport {
            x: 10.0,
            y: 20.5,
            zoom: 0.75,
        };
        assert_eq!(
            viewport.transform_css(),
            "translate(10.00px, 20.50px) scale(0.7500)"
        );
    }
}
