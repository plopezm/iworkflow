//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"regexp"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// runtimeIDPattern accepts lowercase letters and digits and dashes, never at the start or the end.
var runtimeIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// RuntimeID names an agent runtime (for example "pi", "claude", "cursor").
type RuntimeID struct {
	value string
}

// ParseRuntimeID validates a runtime slug; anything else is InvalidRuntime.
func ParseRuntimeID(raw string) (RuntimeID, error) {
	if !runtimeIDPattern.MatchString(raw) {
		return RuntimeID{}, werrors.InvalidRuntime(raw)
	}
	return RuntimeID{value: raw}, nil
}

// String returns the slug.
func (r RuntimeID) String() string {
	return r.value
}

// LaunchModel is the orchestrator model to use for one launch (--provider / --model).
type LaunchModel struct {
	Provider string
	Model    string
}

// LaunchOptions are the options applied to one agent launch. A nil Model or Thinking lets the agent
// use its defaults. Thinking is the lowercase pi identifier (for example "high").
type LaunchOptions struct {
	// Runtime is the agent runtime to use; nil uses the default runtime.
	Runtime  *RuntimeID
	Model    *LaunchModel
	Thinking *string
	// Limits are the RAM and CPU limits of the agent process (Linux with systemd only).
	Limits ResourceLimits
	// LimitsDisabled is the general settings switch: when true no limit is applied.
	LimitsDisabled bool
	// AddDirs are the absolute paths of the related projects the agent may read. Only the claude runtime turns them
	// into a flag (--add-dir); pi and cursor receive no flag and learn the paths from the prompt section only.
	AddDirs []string
}

// WithTaskLimits applies the task's own limits over the global ones, per resource.
func (o LaunchOptions) WithTaskLimits(task ResourceLimits) LaunchOptions {
	if o.LimitsDisabled {
		o.Limits = ResourceLimits{}
	} else {
		o.Limits = task.Or(o.Limits)
	}
	return o
}

// RestrictedTo drops the model and the thinking level that the runtime does not support.
func (o LaunchOptions) RestrictedTo(capabilities AgentCapabilities) LaunchOptions {
	if !capabilities.ModelSelection {
		o.Model = nil
	}
	if !capabilities.ThinkingLevels {
		o.Thinking = nil
	}
	return o
}
