use serde::{Deserialize, Serialize};

/// Arista del tablero: `successor_id` espera a `predecessor_id`. `trigger` es el evento de la
/// predecesora que la dispara (por ahora siempre `merged`).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TaskDependencyDto {
    pub predecessor_id: String,
    pub successor_id: String,
    pub trigger: String,
}

/// Posición de un nodo en el lienzo.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct NodePositionDto {
    pub task_id: String,
    pub x: f64,
    pub y: f64,
}

/// Grafo guardado de un workspace.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
pub struct TaskGraphDto {
    pub dependencies: Vec<TaskDependencyDto>,
    pub positions: Vec<NodePositionDto>,
}

#[cfg(feature = "ssr")]
impl From<crate::modules::tasks::domain::models::task_graph::TaskDependency> for TaskDependencyDto {
    fn from(dependency: crate::modules::tasks::domain::models::task_graph::TaskDependency) -> Self {
        Self {
            predecessor_id: dependency.predecessor.to_string(),
            successor_id: dependency.successor.to_string(),
            trigger: dependency.trigger.as_str().to_string(),
        }
    }
}

#[cfg(feature = "ssr")]
impl From<crate::modules::tasks::domain::models::task_graph::TaskGraph> for TaskGraphDto {
    fn from(graph: crate::modules::tasks::domain::models::task_graph::TaskGraph) -> Self {
        Self {
            dependencies: graph
                .dependencies
                .into_iter()
                .map(TaskDependencyDto::from)
                .collect(),
            positions: graph
                .positions
                .into_iter()
                .map(|(id, position)| NodePositionDto {
                    task_id: id.to_string(),
                    x: position.x,
                    y: position.y,
                })
                .collect(),
        }
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::task::TaskId;
    use crate::modules::tasks::domain::models::task_graph::{
        DependencyTrigger, NodePosition, TaskDependency, TaskGraph,
    };

    #[test]
    fn maps_graph_to_dto() {
        let (a, b): (TaskId, TaskId) = (TaskId::new(), TaskId::new());
        let dto: TaskGraphDto = TaskGraph {
            dependencies: vec![TaskDependency {
                predecessor: a,
                successor: b,
                workspace_id: "w".to_string(),
                trigger: DependencyTrigger::Merged,
            }],
            positions: vec![(a, NodePosition { x: 1.5, y: -2.0 })],
        }
        .into();

        assert_eq!(
            dto.dependencies,
            vec![TaskDependencyDto {
                predecessor_id: a.to_string(),
                successor_id: b.to_string(),
                trigger: "merged".to_string(),
            }]
        );
        assert_eq!(
            dto.positions,
            vec![NodePositionDto {
                task_id: a.to_string(),
                x: 1.5,
                y: -2.0
            }]
        );
    }
}
