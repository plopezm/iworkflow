//! Máquina de estados pura de los gestos del lienzo (arrastrar el fondo, mover un nodo,
//! conectar nodos). Recibe puntos de pantalla y devuelve qué hay que cambiar; el estado de
//! la pantalla solo aplica el resultado.

use crate::modules::tasks::presentation::state::graph_geometry::{Point, Viewport, node_at};

/// Desplazamiento (píxeles de pantalla) a partir del cual un gesto deja de ser un clic.
pub const DRAG_THRESHOLD: f64 = 4.0;

/// Gesto en curso.
#[derive(Debug, Clone, PartialEq, Default)]
pub enum DragMode {
    #[default]
    Idle,
    /// Arrastre del fondo: desplaza la vista.
    Pan {
        origin: Point,
        viewport: Viewport,
        moved: bool,
    },
    /// Arrastre de un nodo.
    Node {
        id: String,
        /// Puntero (en coordenadas de lienzo) al empezar.
        pointer: Point,
        /// Posición del nodo al empezar.
        start: Point,
        /// Puntero (en pantalla) al empezar, para medir el umbral.
        origin: Point,
        moved: bool,
    },
    /// Arista en construcción desde el puerto de salida de `from`.
    Connect {
        from: String,
        /// Cursor en coordenadas de lienzo.
        cursor: Point,
        /// Nodo sobre el que está el cursor, si lo hay.
        target: Option<String>,
    },
}

/// Cambio visual que provoca un movimiento del puntero.
#[derive(Debug, Clone, PartialEq)]
pub enum DragEffect {
    None,
    SetViewport(Viewport),
    MoveNode(String, Point),
}

/// Efecto de soltar el puntero.
#[derive(Debug, Clone, PartialEq)]
pub enum DragOutcome {
    None,
    /// Clic en el fondo: limpia la selección.
    ClearSelection,
    /// Clic sobre un nodo, sin moverlo.
    SelectNode(String),
    /// Fin del arrastre de un nodo: hay que guardar su posición.
    CommitNode(String, Point),
    /// Arista soltada sobre otro nodo.
    Connect {
        from: String,
        to: String,
    },
}

fn distance(a: Point, b: Point) -> f64 {
    ((a.x - b.x).powi(2) + (a.y - b.y).powi(2)).sqrt()
}

impl DragMode {
    pub fn pan(origin: Point, viewport: Viewport) -> Self {
        Self::Pan {
            origin,
            viewport,
            moved: false,
        }
    }

    pub fn node(id: String, screen: Point, start: Point, viewport: &Viewport) -> Self {
        Self::Node {
            id,
            pointer: viewport.screen_to_canvas(screen),
            start,
            origin: screen,
            moved: false,
        }
    }

    pub fn connect(from: String, screen: Point, viewport: &Viewport) -> Self {
        Self::Connect {
            from,
            cursor: viewport.screen_to_canvas(screen),
            target: None,
        }
    }

    pub fn is_idle(&self) -> bool {
        matches!(self, Self::Idle)
    }

    /// Aplica un movimiento del puntero y devuelve el nuevo gesto y su efecto visual.
    /// `nodes` son los nodos con su posición, en orden de pintado.
    pub fn moved(
        self,
        screen: Point,
        viewport: &Viewport,
        nodes: &[(String, Point)],
    ) -> (DragMode, DragEffect) {
        match self {
            Self::Idle => (Self::Idle, DragEffect::None),
            Self::Pan {
                origin,
                viewport: start,
                moved,
            } => {
                let dx: f64 = screen.x - origin.x;
                let dy: f64 = screen.y - origin.y;
                let moved: bool = moved || distance(screen, origin) >= DRAG_THRESHOLD;
                let effect: DragEffect = if moved {
                    DragEffect::SetViewport(start.panned(dx, dy))
                } else {
                    DragEffect::None
                };
                (
                    Self::Pan {
                        origin,
                        viewport: start,
                        moved,
                    },
                    effect,
                )
            }
            Self::Node {
                id,
                pointer,
                start,
                origin,
                moved,
            } => {
                let moved: bool = moved || distance(screen, origin) >= DRAG_THRESHOLD;
                let effect: DragEffect = if moved {
                    let now: Point = viewport.screen_to_canvas(screen);
                    DragEffect::MoveNode(
                        id.clone(),
                        Point::new(start.x + now.x - pointer.x, start.y + now.y - pointer.y),
                    )
                } else {
                    DragEffect::None
                };
                (
                    Self::Node {
                        id,
                        pointer,
                        start,
                        origin,
                        moved,
                    },
                    effect,
                )
            }
            Self::Connect { from, .. } => {
                let cursor: Point = viewport.screen_to_canvas(screen);
                let target: Option<String> =
                    node_at(cursor, nodes).filter(|id: &String| *id != from);
                (
                    Self::Connect {
                        from,
                        cursor,
                        target,
                    },
                    DragEffect::None,
                )
            }
        }
    }

    /// Resuelve el gesto al soltar el puntero en `screen`.
    pub fn finished(
        self,
        screen: Point,
        viewport: &Viewport,
        nodes: &[(String, Point)],
        final_position: impl Fn(&str) -> Option<Point>,
    ) -> DragOutcome {
        match self {
            Self::Idle => DragOutcome::None,
            Self::Pan { moved, .. } => {
                if moved {
                    DragOutcome::None
                } else {
                    DragOutcome::ClearSelection
                }
            }
            Self::Node { id, moved, .. } => {
                if !moved {
                    return DragOutcome::SelectNode(id);
                }
                match final_position(&id) {
                    Some(position) => DragOutcome::CommitNode(id, position),
                    None => DragOutcome::None,
                }
            }
            Self::Connect { from, .. } => {
                let cursor: Point = viewport.screen_to_canvas(screen);
                match node_at(cursor, nodes) {
                    Some(to) if to != from => DragOutcome::Connect { from, to },
                    _ => DragOutcome::None,
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::presentation::state::graph_geometry::NODE_WIDTH;

    fn nodes() -> Vec<(String, Point)> {
        vec![
            ("a".to_string(), Point::new(0.0, 0.0)),
            ("b".to_string(), Point::new(NODE_WIDTH + 100.0, 0.0)),
        ]
    }

    #[test]
    fn small_pan_movements_are_still_a_click() {
        let viewport: Viewport = Viewport::default();
        let mode: DragMode = DragMode::pan(Point::new(10.0, 10.0), viewport);
        let (mode, effect): (DragMode, DragEffect) =
            mode.moved(Point::new(11.0, 11.0), &viewport, &nodes());
        assert_eq!(effect, DragEffect::None);
        assert_eq!(
            mode.finished(Point::new(11.0, 11.0), &viewport, &nodes(), |_| None),
            DragOutcome::ClearSelection
        );
    }

    #[test]
    fn panning_offsets_the_starting_viewport() {
        let viewport: Viewport = Viewport {
            x: 5.0,
            y: 5.0,
            zoom: 1.0,
        };
        let mode: DragMode = DragMode::pan(Point::new(100.0, 100.0), viewport);
        let (mode, effect): (DragMode, DragEffect) =
            mode.moved(Point::new(130.0, 90.0), &viewport, &[]);
        assert_eq!(
            effect,
            DragEffect::SetViewport(Viewport {
                x: 35.0,
                y: -5.0,
                zoom: 1.0
            })
        );
        assert_eq!(
            mode.finished(Point::new(130.0, 90.0), &viewport, &[], |_| None),
            DragOutcome::None
        );
    }

    #[test]
    fn dragging_a_node_moves_it_in_canvas_units() {
        let viewport: Viewport = Viewport {
            x: 50.0,
            y: 0.0,
            zoom: 0.5,
        };
        let mode: DragMode = DragMode::node(
            "a".to_string(),
            Point::new(60.0, 20.0),
            Point::new(100.0, 100.0),
            &viewport,
        );
        let (mode, effect): (DragMode, DragEffect) =
            mode.moved(Point::new(80.0, 30.0), &viewport, &nodes());
        // 20 px de pantalla a zoom 0,5 son 40 unidades de lienzo.
        assert_eq!(
            effect,
            DragEffect::MoveNode("a".to_string(), Point::new(140.0, 120.0))
        );
        let outcome: DragOutcome =
            mode.finished(Point::new(80.0, 30.0), &viewport, &nodes(), |id: &str| {
                (id == "a").then_some(Point::new(140.0, 120.0))
            });
        assert_eq!(
            outcome,
            DragOutcome::CommitNode("a".to_string(), Point::new(140.0, 120.0))
        );
    }

    #[test]
    fn clicking_a_node_without_moving_selects_it_and_saves_nothing() {
        let viewport: Viewport = Viewport::default();
        let mode: DragMode = DragMode::node(
            "a".to_string(),
            Point::new(5.0, 5.0),
            Point::default(),
            &viewport,
        );
        let (mode, effect): (DragMode, DragEffect) =
            mode.moved(Point::new(6.0, 5.0), &viewport, &nodes());
        assert_eq!(effect, DragEffect::None);
        assert_eq!(
            mode.finished(Point::new(6.0, 5.0), &viewport, &nodes(), |_| None),
            DragOutcome::SelectNode("a".to_string())
        );
    }

    #[test]
    fn dropping_a_connection_on_another_node_creates_the_edge() {
        let viewport: Viewport = Viewport::default();
        let mode: DragMode =
            DragMode::connect("a".to_string(), Point::new(NODE_WIDTH, 70.0), &viewport);
        let over_b: Point = Point::new(NODE_WIDTH + 150.0, 40.0);
        let (mode, _): (DragMode, DragEffect) = mode.moved(over_b, &viewport, &nodes());
        assert!(matches!(
            &mode,
            DragMode::Connect { target: Some(t), .. } if t == "b"
        ));
        assert_eq!(
            mode.finished(over_b, &viewport, &nodes(), |_| None),
            DragOutcome::Connect {
                from: "a".to_string(),
                to: "b".to_string()
            }
        );
    }

    #[test]
    fn dropping_a_connection_on_itself_or_the_void_does_nothing() {
        let viewport: Viewport = Viewport::default();
        let mode: DragMode =
            DragMode::connect("a".to_string(), Point::new(NODE_WIDTH, 70.0), &viewport);
        let (mode_self, _): (DragMode, DragEffect) =
            mode.clone()
                .moved(Point::new(20.0, 20.0), &viewport, &nodes());
        assert!(matches!(&mode_self, DragMode::Connect { target: None, .. }));
        assert_eq!(
            mode_self.finished(Point::new(20.0, 20.0), &viewport, &nodes(), |_| None),
            DragOutcome::None
        );
        assert_eq!(
            mode.finished(Point::new(-900.0, -900.0), &viewport, &nodes(), |_| None),
            DragOutcome::None
        );
    }

    #[test]
    fn idle_ignores_everything() {
        let viewport: Viewport = Viewport::default();
        assert!(DragMode::Idle.is_idle());
        let (mode, effect): (DragMode, DragEffect) =
            DragMode::Idle.moved(Point::default(), &viewport, &[]);
        assert!(mode.is_idle());
        assert_eq!(effect, DragEffect::None);
        assert_eq!(
            DragMode::Idle.finished(Point::default(), &viewport, &[], |_| None),
            DragOutcome::None
        );
    }
}
