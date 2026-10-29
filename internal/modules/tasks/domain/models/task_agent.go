package models

import "strings"

// TaskAgentOptions holds task-specific overrides for the agent execution.
// A nil or empty field means "use the default from global settings / agent defaults".
type TaskAgentOptions struct {
	Runtime       *RuntimeID
	ModelProvider *string
	ModelID       *string
	Thinking      *string
	Profile       *string
}

// IsZero reports whether no custom agent options are set.
func (o TaskAgentOptions) IsZero() bool {
	return o.Runtime == nil && o.ModelProvider == nil && o.ModelID == nil && o.Thinking == nil && o.Profile == nil
}

// LaunchModel returns the launch model if ModelID is set.
func (o TaskAgentOptions) LaunchModel() *LaunchModel {
	if o.ModelID == nil || *o.ModelID == "" {
		return nil
	}
	provider := ""
	if o.ModelProvider != nil {
		provider = *o.ModelProvider
	}
	return &LaunchModel{
		Provider: provider,
		Model:    *o.ModelID,
	}
}

// Validated trims string fields and ensures valid values.
func (o TaskAgentOptions) Validated() (TaskAgentOptions, error) {
	var runtime *RuntimeID
	if o.Runtime != nil {
		raw := strings.TrimSpace(o.Runtime.String())
		if raw != "" {
			parsed, err := ParseRuntimeID(raw)
			if err != nil {
				return TaskAgentOptions{}, err
			}
			runtime = &parsed
		}
	}
	var modelProvider *string
	if o.ModelProvider != nil {
		trimmed := strings.TrimSpace(*o.ModelProvider)
		if trimmed != "" {
			modelProvider = &trimmed
		}
	}
	var modelID *string
	if o.ModelID != nil {
		trimmed := strings.TrimSpace(*o.ModelID)
		if trimmed != "" {
			modelID = &trimmed
		}
	}
	var thinking *string
	if o.Thinking != nil {
		trimmed := strings.TrimSpace(*o.Thinking)
		if trimmed != "" {
			thinking = &trimmed
		}
	}
	var profile *string
	if o.Profile != nil {
		trimmed := strings.TrimSpace(*o.Profile)
		if trimmed != "" {
			profile = &trimmed
		}
	}
	return TaskAgentOptions{
		Runtime:       runtime,
		ModelProvider: modelProvider,
		ModelID:       modelID,
		Thinking:      thinking,
		Profile:       profile,
	}, nil
}
