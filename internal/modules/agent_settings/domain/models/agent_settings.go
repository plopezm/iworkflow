package models

// AgentSettings are the global agent settings. A nil field means "use the pi/gentle defaults".
type AgentSettings struct {
	Runtime  *AgentRuntime
	Model    *ModelRef
	Profile  *ProfileName
	Thinking *ThinkingLevel
	// MemoryLimitMB caps the RAM of each task in MiB; nil means no limit.
	MemoryLimitMB *uint32
	// CPULimitPercent caps the CPU of each task in % of one core; nil means no limit.
	CPULimitPercent *uint32
	// LimitsDisabled is the master switch: when true, no limit (global or per task) applies.
	LimitsDisabled bool
}
