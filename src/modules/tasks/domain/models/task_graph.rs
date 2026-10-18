use std::collections::{HashMap, HashSet};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};

/// Evento de la predecesora que dispara a la sucesora. Es el punto de extensión de las
/// condiciones de arista, que llegarán en otra feature.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum DependencyTrigger {
    /// La predecesora se ha integrado en su rama base.
    #[default]
    Merged,
}

impl DependencyTrigger {
    pub fn as_str(&self) -> &'static str {
        match self {
            DependencyTrigger::Merged => "merged",
        }
    }

    pub fn parse(value: &str) -> Result<Self, TaskError> {
        match value {
            "merged" => Ok(DependencyTrigger::Merged),
            other => Err(TaskError::InvalidTrigger(other.to_string())),
        }
    }

    /// `true` si el estado de la predecesora satisface el disparador.
    pub fn is_satisfied_by(&self, status: TaskStatus) -> bool {
        match self {
            DependencyTrigger::Merged => status == TaskStatus::Merged,
        }
    }
}

/// Arista del grafo: `successor` espera a `predecessor`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TaskDependency {
    pub predecessor: TaskId,
    pub successor: TaskId,
    pub workspace_id: String,
    pub trigger: DependencyTrigger,
}

/// Coordenadas de un nodo en el lienzo del tablero.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct NodePosition {
    pub x: f64,
    pub y: f64,
}

impl NodePosition {
    pub fn new(x: f64, y: f64) -> Result<Self, TaskError> {
        if x.is_finite() && y.is_finite() {
            Ok(Self { x, y })
        } else {
            Err(TaskError::InvalidPosition)
        }
    }
}

/// Lectura del grafo de un workspace: aristas y posiciones guardadas de sus nodos.
#[derive(Debug, Clone, PartialEq, Default)]
pub struct TaskGraph {
    pub dependencies: Vec<TaskDependency>,
    pub positions: Vec<(TaskId, NodePosition)>,
}

/// Comprueba que `candidate` se puede añadir a `existing` (las aristas de su workspace):
/// sin autoarista, sin duplicado y sin ciclo.
pub fn validate_new_dependency(
    existing: &[TaskDependency],
    candidate: &TaskDependency,
) -> Result<(), TaskError> {
    if candidate.predecessor == candidate.successor {
        return Err(TaskError::SelfDependency);
    }
    if existing.iter().any(|edge: &TaskDependency| {
        edge.predecessor == candidate.predecessor && edge.successor == candidate.successor
    }) {
        return Err(TaskError::DuplicateDependency);
    }
    // Un ciclo aparece si la predecesora ya es alcanzable desde la sucesora.
    if reaches(existing, candidate.successor, candidate.predecessor) {
        return Err(TaskError::DependencyCycle);
    }
    Ok(())
}

fn reaches(edges: &[TaskDependency], from: TaskId, target: TaskId) -> bool {
    let mut next: HashMap<TaskId, Vec<TaskId>> = HashMap::new();
    for edge in edges {
        next.entry(edge.predecessor)
            .or_default()
            .push(edge.successor);
    }
    let mut seen: HashSet<TaskId> = HashSet::new();
    let mut stack: Vec<TaskId> = vec![from];
    while let Some(current) = stack.pop() {
        if current == target {
            return true;
        }
        if seen.insert(current)
            && let Some(children) = next.get(&current)
        {
            stack.extend(children.iter().copied());
        }
    }
    false
}

/// Tareas en `Backlog` con al menos una predecesora cuyas predecesoras satisfacen todas su
/// disparador: las que deben arrancar ahora. Las tareas sin predecesoras se arrancan a mano.
pub fn startable_tasks(tasks: &[Task], edges: &[TaskDependency]) -> Vec<TaskId> {
    let status_of: HashMap<TaskId, TaskStatus> = tasks
        .iter()
        .map(|task: &Task| (task.id, task.status))
        .collect();
    let mut incoming: HashMap<TaskId, Vec<&TaskDependency>> = HashMap::new();
    for edge in edges {
        incoming.entry(edge.successor).or_default().push(edge);
    }
    tasks
        .iter()
        .filter(|task: &&Task| task.status == TaskStatus::Backlog)
        .filter(|task: &&Task| {
            incoming
                .get(&task.id)
                .is_some_and(|deps: &Vec<&TaskDependency>| {
                    deps.iter().all(|edge: &&TaskDependency| {
                        status_of
                            .get(&edge.predecessor)
                            .is_some_and(|status: &TaskStatus| {
                                edge.trigger.is_satisfied_by(*status)
                            })
                    })
                })
        })
        .map(|task: &Task| task.id)
        .collect()
}

/// Tareas en `Backlog` que nunca arrancarán solas porque alguna predecesora está `Discarded`
/// (descartada después de crear la arista). El usuario las desbloquea quitando esa arista.
pub fn blocked_by_discarded(tasks: &[Task], edges: &[TaskDependency]) -> Vec<TaskId> {
    let status_of: HashMap<TaskId, TaskStatus> = tasks
        .iter()
        .map(|task: &Task| (task.id, task.status))
        .collect();
    tasks
        .iter()
        .filter(|task: &&Task| task.status == TaskStatus::Backlog)
        .filter(|task: &&Task| {
            edges.iter().any(|edge: &TaskDependency| {
                edge.successor == task.id
                    && status_of.get(&edge.predecessor) == Some(&TaskStatus::Discarded)
            })
        })
        .map(|task: &Task| task.id)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::application::test_support;

    fn edge(predecessor: TaskId, successor: TaskId) -> TaskDependency {
        TaskDependency {
            predecessor,
            successor,
            workspace_id: "w1".to_string(),
            trigger: DependencyTrigger::Merged,
        }
    }

    fn task(status: TaskStatus) -> Task {
        test_support::task(status)
    }

    #[test]
    fn trigger_round_trips_and_rejects_unknown() {
        assert_eq!(
            DependencyTrigger::parse(DependencyTrigger::Merged.as_str()),
            Ok(DependencyTrigger::Merged)
        );
        assert_eq!(
            DependencyTrigger::parse("nope"),
            Err(TaskError::InvalidTrigger("nope".to_string()))
        );
    }

    #[test]
    fn position_rejects_non_finite_coordinates() {
        assert!(NodePosition::new(1.0, -2.5).is_ok());
        assert_eq!(
            NodePosition::new(f64::NAN, 0.0),
            Err(TaskError::InvalidPosition)
        );
        assert_eq!(
            NodePosition::new(0.0, f64::INFINITY),
            Err(TaskError::InvalidPosition)
        );
    }

    #[test]
    fn rejects_self_duplicate_and_cycles() {
        let (a, b, c): (TaskId, TaskId, TaskId) = (TaskId::new(), TaskId::new(), TaskId::new());
        let existing: Vec<TaskDependency> = vec![edge(a, b), edge(b, c)];
        assert_eq!(
            validate_new_dependency(&existing, &edge(a, a)),
            Err(TaskError::SelfDependency)
        );
        assert_eq!(
            validate_new_dependency(&existing, &edge(a, b)),
            Err(TaskError::DuplicateDependency)
        );
        assert_eq!(
            validate_new_dependency(&existing, &edge(c, a)),
            Err(TaskError::DependencyCycle)
        );
        // Paralelo y diamante son válidos.
        assert_eq!(validate_new_dependency(&existing, &edge(a, c)), Ok(()));
    }

    #[test]
    fn starts_backlog_task_only_when_every_predecessor_is_merged() {
        let merged: Task = task(TaskStatus::Merged);
        let reviewing: Task = task(TaskStatus::AwaitingReview);
        let ready: Task = task(TaskStatus::Backlog);
        let waiting: Task = task(TaskStatus::Backlog);
        let root: Task = task(TaskStatus::Backlog);
        let edges: Vec<TaskDependency> = vec![
            edge(merged.id, ready.id),
            edge(merged.id, waiting.id),
            edge(reviewing.id, waiting.id),
        ];
        let tasks: Vec<Task> = vec![merged.clone(), reviewing, ready.clone(), waiting, root];
        assert_eq!(startable_tasks(&tasks, &edges), vec![ready.id]);
    }

    #[test]
    fn already_started_successors_are_not_restarted() {
        let merged: Task = task(TaskStatus::Merged);
        let running: Task = task(TaskStatus::Running);
        let edges: Vec<TaskDependency> = vec![edge(merged.id, running.id)];
        assert!(startable_tasks(&[merged, running], &edges).is_empty());
    }

    #[test]
    fn backlog_successor_with_discarded_predecessor_is_blocked() {
        let discarded: Task = task(TaskStatus::Discarded);
        let merged: Task = task(TaskStatus::Merged);
        let stuck: Task = task(TaskStatus::Backlog);
        let fine: Task = task(TaskStatus::Backlog);
        let running: Task = task(TaskStatus::Running);
        let edges: Vec<TaskDependency> = vec![
            edge(discarded.id, stuck.id),
            edge(merged.id, stuck.id),
            edge(merged.id, fine.id),
            edge(discarded.id, running.id),
        ];
        let stuck_id: TaskId = stuck.id;
        let tasks: Vec<Task> = vec![discarded, merged, stuck, fine, running];

        assert_eq!(blocked_by_discarded(&tasks, &edges), vec![stuck_id]);
        assert!(
            startable_tasks(&tasks, &edges)
                .iter()
                .all(|id: &TaskId| *id != stuck_id)
        );
    }
}
