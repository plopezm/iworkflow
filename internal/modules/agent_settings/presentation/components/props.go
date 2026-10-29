// Package components holds the templ components of the agent settings module. They render the
// props they receive and contain no logic: the state package builds the props, and every text in
// them is already translated.
package components

import "iworkflow/internal/core/components"

// Element ids the htmx requests swap into. They are part of the contract with the handlers.
const (
	// BarID is the root of the agent settings bar.
	BarID = "agent-settings-bar"
	// LimitsID is the root of the resource limits card.
	LimitsID = "agent-settings-limits"
)

// BarSlot is one selector of the bar. Err, when set, replaces the selector with the error text
// (only that selector failed to load its options).
type BarSlot struct {
	Select components.OptionSelectProps
	Err    string
}

// AgentSettingsBarProps configures the agent settings bar: the agent implementation, the model, the
// reasoning level and the profile of the next tasks.
//
// Error is set when the stored settings cannot be read; the bar then shows only that error.
// Tooltip is the hover text of the bar and Footer the note under the selectors; both are translated.
type AgentSettingsBarProps struct {
	Runtime        BarSlot
	Model          BarSlot
	Thinking       BarSlot
	Profile        BarSlot
	ActiveRuntime  string
	ActiveModel    string
	ActiveThinking string
	ActiveProfile  string
	StatusTitle    string
	Error          string
	Tooltip        string
	Footer         string
}

// ResourceLimitsProps configures the card with the RAM and CPU caps of each task's agent.
//
// Loaded is false when the stored settings could not be read; the card then shows only Error. When
// it is true, MemoryMB and CPUPercent are the form text of each cap ("" means no limit) and Enabled
// is the state of the master switch: while it is off both inputs are disabled. Notice reports a
// failed change and is shown above the controls, which keep the stored values. Every text is
// translated by the state package.
type ResourceLimitsProps struct {
	Loaded      bool
	MemoryMB    string
	CPUPercent  string
	Enabled     bool
	Title       string
	Help        string
	Switch      string
	NoLimitText string
	Error       string
	Notice      string
}
