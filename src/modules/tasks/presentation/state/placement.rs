//! Transiciones puras de las posiciones fijadas en la sesión («overrides»): colocación
//! optimista, confirmación por el servidor y vuelta atrás si falla el guardado. El estado de
//! pantalla solo aplica el resultado.

use std::collections::HashMap;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskDto;
use crate::modules::tasks::presentation::state::graph_geometry::Point;

/// Diferencia (unidades de lienzo) por debajo de la cual dos posiciones son la misma.
const POSITION_EPSILON: f64 = 0.01;

fn same_position(a: Point, b: Point) -> bool {
    (a.x - b.x).abs() < POSITION_EPSILON && (a.y - b.y).abs() < POSITION_EPSILON
}

/// Overrides con `id` fijado de forma optimista en `position`.
pub fn place(
    overrides: &HashMap<String, Point>,
    id: &str,
    position: Point,
) -> HashMap<String, Point> {
    let mut next: HashMap<String, Point> = overrides.clone();
    next.insert(id.to_string(), position);
    next
}

/// Overrides sin `id`: la tarea vuelve a su posición guardada (o a estar suelta) cuando el
/// servidor rechaza el guardado.
pub fn roll_back(overrides: &HashMap<String, Point>, id: &str) -> HashMap<String, Point> {
    let mut next: HashMap<String, Point> = overrides.clone();
    next.remove(id);
    next
}

/// Estado de pantalla al sacar `id` del grafo: sin posición provisional y, si era el nodo
/// seleccionado, sin selección. Se aplica al lanzar la acción, no al terminar: así dos desacoples
/// seguidos limpian cada uno lo suyo aunque la acción solo conserve el resultado del último.
pub fn after_detach(
    overrides: &HashMap<String, Point>,
    selected: Option<&str>,
    id: &str,
) -> (HashMap<String, Point>, Option<String>) {
    let kept: Option<String> = selected
        .filter(|current: &&str| *current != id)
        .map(str::to_string);
    (roll_back(overrides, id), kept)
}

/// Overrides que el servidor aún no ha confirmado: se descartan los que ya coinciden con la
/// posición guardada. Una carga más antigua que el guardado (otra posición) no los toca.
pub fn unconfirmed(
    overrides: &HashMap<String, Point>,
    saved: &HashMap<String, Point>,
) -> HashMap<String, Point> {
    overrides
        .iter()
        .filter(|(id, position): &(&String, &Point)| {
            !saved
                .get(id.as_str())
                .is_some_and(|stored: &Point| same_position(*stored, **position))
        })
        .map(|(id, position): (&String, &Point)| (id.clone(), *position))
        .collect()
}

/// Solo se puede colocar una tarea que sigue suelta: si ya es un nodo (o desapareció) mientras
/// se arrastraba, soltarla no debe pisar su posición.
pub fn can_place(task_id: &str, loose: &[TaskDto]) -> bool {
    loose.iter().any(|t: &TaskDto| t.id == task_id)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
        TaskStageDto, TaskStatusDto,
    };

    fn at(x: f64, y: f64) -> Point {
        Point::new(x, y)
    }

    fn map(list: &[(&str, Point)]) -> HashMap<String, Point> {
        list.iter()
            .map(|(id, p): &(&str, Point)| ((*id).to_string(), *p))
            .collect()
    }

    fn loose_task(id: &str) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: "w".to_string(),
            title: id.to_string(),
            prompt: String::new(),
            branch: String::new(),
            base_branch: "main".to_string(),
            worktree_path: String::new(),
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
            created_at: String::new(),
            updated_at: String::new(),
        }
    }

    #[test]
    fn detaching_drops_only_that_override_and_selection() {
        let overrides: HashMap<String, Point> = map(&[("a", at(1.0, 2.0)), ("b", at(3.0, 4.0))]);

        let (next, selected) = after_detach(&overrides, Some("a"), "a");
        assert_eq!(next, map(&[("b", at(3.0, 4.0))]));
        assert_eq!(selected, None);

        let (next, selected) = after_detach(&overrides, Some("b"), "a");
        assert_eq!(next, map(&[("b", at(3.0, 4.0))]));
        assert_eq!(selected.as_deref(), Some("b"));

        let (next, selected) = after_detach(&overrides, None, "missing");
        assert_eq!(next, overrides);
        assert_eq!(selected, None);
    }

    #[test]
    fn placing_is_optimistic_and_keeps_other_overrides() {
        let before: HashMap<String, Point> = map(&[("a", at(1.0, 1.0))]);
        let after: HashMap<String, Point> = place(&before, "b", at(5.0, 6.0));
        assert_eq!(after, map(&[("a", at(1.0, 1.0)), ("b", at(5.0, 6.0))]));
        // La entrada no cambia.
        assert_eq!(before.len(), 1);
        assert_eq!(place(&after, "b", at(7.0, 7.0))["b"], at(7.0, 7.0));
    }

    #[test]
    fn rolling_back_drops_only_the_failed_task() {
        let before: HashMap<String, Point> = map(&[("a", at(1.0, 1.0)), ("b", at(5.0, 6.0))]);
        assert_eq!(roll_back(&before, "b"), map(&[("a", at(1.0, 1.0))]));
        assert_eq!(roll_back(&before, "zz"), before);
    }

    #[test]
    fn detaching_drops_the_pending_override_so_the_task_is_loose_again() {
        let pending: HashMap<String, Point> = map(&[("a", at(1.0, 1.0))]);
        assert!(roll_back(&pending, "a").is_empty());
    }

    #[test]
    fn a_load_that_matches_the_override_confirms_it() {
        let overrides: HashMap<String, Point> = map(&[("a", at(1.0, 1.0)), ("b", at(5.0, 6.0))]);
        let saved: HashMap<String, Point> = map(&[("a", at(1.0, 1.0)), ("b", at(0.0, 0.0))]);
        // `a` ya está guardada; `b` sigue con la posición vieja (carga anterior al guardado).
        assert_eq!(unconfirmed(&overrides, &saved), map(&[("b", at(5.0, 6.0))]));
        assert!(unconfirmed(&overrides, &overrides).is_empty());
        assert_eq!(unconfirmed(&overrides, &HashMap::new()), overrides);
    }

    #[test]
    fn only_tasks_still_loose_can_be_placed() {
        let loose: Vec<TaskDto> = vec![loose_task("a")];
        assert!(can_place("a", &loose));
        assert!(!can_place("b", &loose));
        assert!(!can_place("a", &[]));
    }
}
