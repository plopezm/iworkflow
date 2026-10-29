package outbound

import "context"

// AgentModelOption is an orchestrator model available for an agent runtime.
type AgentModelOption struct {
	Provider string
	Model    string
}

// AgentCatalogInfo represents the configuration options available for a specific agent runtime,
// indicating which features (models, thinking/effort levels, profiles) the agent supports.
type AgentCatalogInfo struct {
	DefaultRuntime   string
	Runtimes         []string
	SelectedRuntime  string
	SupportsModels   bool
	Models           []AgentModelOption
	DefaultModel     string
	SupportsThinking bool
	ThinkingLevels   []string
	DefaultThinking  string
	SupportsProfiles bool
	Profiles         []string
	ActiveProfile    string
	DefaultProfile   string
}

// AgentCatalogPort provides runtime metadata, models, thinking levels, and profiles for task forms,
// abstracting the agent_settings module away from the tasks domain.
type AgentCatalogPort interface {
	CatalogForRuntime(ctx context.Context, runtime string) (AgentCatalogInfo, error)
}
