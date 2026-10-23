package models

// LaunchPreferences are the preferences resolved for one launch: the agent runtime, the
// orchestrator model and the reasoning level to apply. A nil field lets pi/gentle-shell use their
// defaults.
type LaunchPreferences struct {
	Runtime         *AgentRuntime
	Model           *ModelRef
	Thinking        *ThinkingLevel
	MemoryLimitMB   *uint32
	CPULimitPercent *uint32
	LimitsDisabled  bool
}
