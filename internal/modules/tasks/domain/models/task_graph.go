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

// ValidateNewDependency reports whether candidate can be added to existing, the edges of its workspace:
// no self edge, no duplicate and no cycle. It only reads existing.
func ValidateNewDependency(existing []TaskDependency, candidate TaskDependency) error {
	if candidate.Predecessor == candidate.Successor {
		return werrors.SelfDependency()
	}
	for _, edge := range existing {
		if edge.Predecessor == candidate.Predecessor && edge.Successor == candidate.Successor {
			return werrors.DuplicateDependency()
		}
	}
	// A cycle appears when the predecessor is already reachable from the successor.
	if reaches(existing, candidate.Successor, candidate.Predecessor) {
		return werrors.DependencyCycle()
	}
	return nil
}

// reaches reports whether target can be reached from one of the edges starting at from.
func reaches(edges []TaskDependency, from TaskID, target TaskID) bool {
	next := make(map[TaskID][]TaskID, len(edges))
	for _, edge := range edges {
		next[edge.Predecessor] = append(next[edge.Predecessor], edge.Successor)
	}
	seen := make(map[TaskID]struct{}, len(edges))
	stack := []TaskID{from}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == target {
			return true
		}
		if _, visited := seen[current]; visited {
			continue
		}
		seen[current] = struct{}{}
		stack = append(stack, next[current]...)
	}
	return false
}

// StartableTasks returns the backlog tasks with at least one predecessor, and whose predecessors all satisfy
// their trigger: the ones that must start now. Tasks without predecessors are started by hand.
func StartableTasks(tasks []Task, edges []TaskDependency) []TaskID {
	statusOf := statusIndex(tasks)
	incoming := make(map[TaskID][]TaskDependency, len(edges))
	for _, edge := range edges {
		incoming[edge.Successor] = append(incoming[edge.Successor], edge)
	}
	var startable []TaskID
	for _, task := range tasks {
		if task.Status != StatusBacklog {
			continue
		}
		deps, hasPredecessors := incoming[task.ID]
		if !hasPredecessors {
			continue
		}
		if allTriggersSatisfied(deps, statusOf) {
			startable = append(startable, task.ID)
		}
	}
	return startable
}

// allTriggersSatisfied reports whether every edge has a known predecessor whose status satisfies its trigger.
func allTriggersSatisfied(deps []TaskDependency, statusOf map[TaskID]TaskStatus) bool {
	for _, edge := range deps {
		status, known := statusOf[edge.Predecessor]
		if !known || !edge.Trigger.IsSatisfiedBy(status) {
			return false
		}
	}
	return true
}

// BlockedByDiscarded returns the backlog tasks that have a discarded predecessor. They never start by
// themselves: the user unblocks them by removing that edge.
func BlockedByDiscarded(tasks []Task, edges []TaskDependency) []TaskID {
	statusOf := statusIndex(tasks)
	var blocked []TaskID
	for _, task := range tasks {
		if task.Status != StatusBacklog {
			continue
		}
		for _, edge := range edges {
			if edge.Successor == task.ID && statusOf[edge.Predecessor] == StatusDiscarded {
				blocked = append(blocked, task.ID)
				break
			}
		}
	}
	return blocked
}

// statusIndex maps every task to its status.
func statusIndex(tasks []Task) map[TaskID]TaskStatus {
	index := make(map[TaskID]TaskStatus, len(tasks))
	for _, task := range tasks {
		index[task.ID] = task.Status
	}
	return index
}
