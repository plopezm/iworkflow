//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"math"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// DependencyTrigger is the event of the predecessor that starts the successor. Merged is the only one
// for now; it is the extension point of future edge conditions.
type DependencyTrigger string

// Triggers.
const DependencyTriggerMerged DependencyTrigger = "merged"

// ParseDependencyTrigger converts a name to a trigger; unknown names are InvalidTrigger.
func ParseDependencyTrigger(value string) (DependencyTrigger, error) {
	if value == string(DependencyTriggerMerged) {
		return DependencyTriggerMerged, nil
	}
	return "", werrors.InvalidTrigger(value)
}

// IsSatisfiedBy reports whether the status of the predecessor satisfies the trigger.
func (t DependencyTrigger) IsSatisfiedBy(status TaskStatus) bool {
	return t == DependencyTriggerMerged && status == StatusMerged
}

// TaskDependency is an edge of the graph: the successor waits for the predecessor.
type TaskDependency struct {
	Predecessor TaskID
	Successor   TaskID
	WorkspaceID string
	Trigger     DependencyTrigger
}

// NodePosition holds the coordinates of a node on the board canvas.
type NodePosition struct {
	X float64
	Y float64
}

// NewNodePosition builds a position; coordinates must be finite numbers.
func NewNodePosition(x float64, y float64) (NodePosition, error) {
	if math.IsInf(x, 0) || math.IsNaN(x) || math.IsInf(y, 0) || math.IsNaN(y) {
		return NodePosition{}, werrors.InvalidPosition()
	}
	return NodePosition{X: x, Y: y}, nil
}

// TaskPosition is the saved position of one task node.
type TaskPosition struct {
	TaskID   TaskID
	Position NodePosition
}

// TaskGraph is the read of the graph of a workspace: its edges and the saved node positions.
type TaskGraph struct {
	Dependencies []TaskDependency
	Positions    []TaskPosition
}
