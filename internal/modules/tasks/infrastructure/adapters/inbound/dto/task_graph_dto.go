package dto

import "iworkflow/internal/modules/tasks/domain/models"

// TaskDependencyDTO is an edge of the board: successor_id waits for predecessor_id. trigger is the event of
// the predecessor that starts the successor (merged, for now).
type TaskDependencyDTO struct {
	PredecessorID string `json:"predecessor_id"`
	SuccessorID   string `json:"successor_id"`
	Trigger       string `json:"trigger"`
}

// NodePositionDTO is the position of a node on the board canvas.
type NodePositionDTO struct {
	TaskID string  `json:"task_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// TaskGraphDTO is the saved graph of one workspace: its edges and the positions of its nodes.
type TaskGraphDTO struct {
	Dependencies []TaskDependencyDTO `json:"dependencies"`
	Positions    []NodePositionDTO   `json:"positions"`
}

// TaskGraphFromDomain maps the graph to its wire form. Both lists are never nil, so an empty graph is sent
// as empty arrays.
func TaskGraphFromDomain(graph models.TaskGraph) TaskGraphDTO {
	dependencies := make([]TaskDependencyDTO, 0, len(graph.Dependencies))
	for _, dependency := range graph.Dependencies {
		dependencies = append(dependencies, TaskDependencyDTO{
			PredecessorID: dependency.Predecessor.String(),
			SuccessorID:   dependency.Successor.String(),
			Trigger:       string(dependency.Trigger),
		})
	}
	positions := make([]NodePositionDTO, 0, len(graph.Positions))
	for _, stored := range graph.Positions {
		positions = append(positions, NodePositionDTO{
			TaskID: stored.TaskID.String(),
			X:      stored.Position.X,
			Y:      stored.Position.Y,
		})
	}
	return TaskGraphDTO{Dependencies: dependencies, Positions: positions}
}
