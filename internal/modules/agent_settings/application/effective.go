package application

import "iworkflow/internal/modules/agent_settings/domain/models"

// effectiveThinking drops a saved thinking level the runtime does not support, so a stale level
// never reaches the agent. nil stays nil.
func effectiveThinking(thinking *models.ThinkingLevel, runtime models.AgentRuntime) *models.ThinkingLevel {
	if thinking == nil || !thinking.IsSupportedBy(runtime) {
		return nil
	}
	return thinking
}
