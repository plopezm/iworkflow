//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// dependencyFromRow converts a stored edge back to the domain. A malformed identifier or an unknown trigger
// means a corrupted row: it is reported, never skipped.
func dependencyFromRow(row gen.TaskDependency) (models.TaskDependency, error) {
	predecessor, err := models.ParseTaskID(row.PredecessorID)
	if err != nil {
		return models.TaskDependency{}, err
	}
	successor, err := models.ParseTaskID(row.SuccessorID)
	if err != nil {
		return models.TaskDependency{}, err
	}
	trigger, err := models.ParseDependencyTrigger(row.TriggerEvent)
	if err != nil {
		return models.TaskDependency{}, err
	}
	return models.TaskDependency{
		Predecessor: predecessor,
		Successor:   successor,
		WorkspaceID: row.WorkspaceID,
		Trigger:     trigger,
	}, nil
}

// dependenciesFromRows converts every stored edge; the first corrupted row fails the read.
func dependenciesFromRows(rows []gen.TaskDependency) ([]models.TaskDependency, error) {
	edges := make([]models.TaskDependency, 0, len(rows))
	for _, row := range rows {
		edge, err := dependencyFromRow(row)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

// positionFromRow converts a stored node position back to the domain, rejecting non-finite coordinates.
func positionFromRow(row gen.TaskPosition) (models.TaskPosition, error) {
	taskID, err := models.ParseTaskID(row.TaskID)
	if err != nil {
		return models.TaskPosition{}, err
	}
	position, err := models.NewNodePosition(row.X, row.Y)
	if err != nil {
		return models.TaskPosition{}, err
	}
	return models.TaskPosition{TaskID: taskID, Position: position}, nil
}
