//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// timestampLayout is the text form the Rust (Diesel) version wrote for TIMESTAMP columns:
// "YYYY-MM-DD HH:MM:SS" plus the fraction only when it is not zero (chrono's %.f). Both versions
// must read each other's rows.
const timestampLayout = "2006-01-02 15:04:05.999999999"

// timestampLayouts are accepted when reading. The last one is the RFC 3339 form some drivers return.
var timestampLayouts = [...]string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999Z07:00",
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(timestampLayout)
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range timestampLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

// pendingDialogJSON is the persisted form of a dialog: {"id": ..., "expires_at": ...|null}.
type pendingDialogJSON struct {
	ID        string     `json:"id"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// encodePendingDialogs serializes the dialogs to the persisted JSON array.
func encodePendingDialogs(dialogs []models.PendingDialog) (string, error) {
	wire := make([]pendingDialogJSON, 0, len(dialogs))
	for _, dialog := range dialogs {
		var expiresAt *time.Time
		if dialog.ExpiresAt != nil {
			utc := dialog.ExpiresAt.UTC()
			expiresAt = &utc
		}
		wire = append(wire, pendingDialogJSON{ID: dialog.ID, ExpiresAt: expiresAt})
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return "", fmt.Errorf("encode pending dialogs: %w", err)
	}
	return string(data), nil
}

// decodePendingDialogs reads the persisted array. It accepts the legacy format (an array of ids as
// strings) besides the current one, as the Rust version does.
func decodePendingDialogs(payload string) ([]models.PendingDialog, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, werrors.Repository(fmt.Sprintf("pending_dialogs inválido: %v", err))
	}
	dialogs := make([]models.PendingDialog, 0, len(raw))
	for _, item := range raw {
		var legacy string
		if err := json.Unmarshal(item, &legacy); err == nil {
			dialogs = append(dialogs, models.NewPendingDialog(legacy, nil))
			continue
		}
		var current pendingDialogJSON
		if err := json.Unmarshal(item, &current); err != nil {
			return nil, werrors.Repository(fmt.Sprintf("pending_dialogs inválido: %v", err))
		}
		dialogs = append(dialogs, models.NewPendingDialog(current.ID, current.ExpiresAt))
	}
	return dialogs, nil
}

// decodePendingDialogsOrEmpty mirrors the Rust `unwrap_or_default`: an unreadable list reads as empty.
func decodePendingDialogsOrEmpty(payload string) []models.PendingDialog {
	dialogs, err := decodePendingDialogs(payload)
	if err != nil {
		return nil
	}
	return dialogs
}

// clampCount converts a counter to the INTEGER column, saturating at the maximum (as the Rust version).
func clampCount(value uint32) int64 {
	return min(int64(value), math.MaxInt32)
}

// limitToColumn stores a limit, or NULL when it is unset or does not fit in the column.
func limitToColumn(value *uint32) *int64 {
	if value == nil || int64(*value) > math.MaxInt32 {
		return nil
	}
	stored := int64(*value)
	return &stored
}

// limitFromColumn reads a limit; negative or NULL values mean no limit.
func limitFromColumn(value *int64) *uint32 {
	if value == nil || *value < 0 || *value > math.MaxUint32 {
		return nil
	}
	limit := uint32(*value)
	return &limit
}

// rowToTask maps a stored row to the domain task. Malformed stored data is a repository error.
func rowToTask(row gen.Task) (models.Task, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return models.Task{}, werrors.Repository(fmt.Sprintf("id inválido %s: %v", row.ID, err))
	}
	status, err := models.ParseTaskStatus(row.Status)
	if err != nil {
		return models.Task{}, err
	}
	priority, err := models.ParseTaskPriority(row.Priority)
	if err != nil {
		return models.Task{}, err
	}
	dialogs, err := decodePendingDialogs(row.PendingDialogs)
	if err != nil {
		return models.Task{}, err
	}
	createdAt, err := parseTimestamp(row.CreatedAt)
	if err != nil {
		return models.Task{}, werrors.Repository(fmt.Sprintf("fecha de creación inválida %s: %v", row.CreatedAt, err))
	}
	updatedAt, err := parseTimestamp(row.UpdatedAt)
	if err != nil {
		return models.Task{}, werrors.Repository(fmt.Sprintf("fecha de actualización inválida %s: %v", row.UpdatedAt, err))
	}
	usage := models.UsageLimitState{Attempts: clampAttempts(row.ResumeAttempts)}
	if row.ResumeAt != nil {
		resumeAt, err := parseTimestamp(*row.ResumeAt)
		if err != nil {
			return models.Task{}, werrors.Repository(fmt.Sprintf("resume_at inválido %s: %v", *row.ResumeAt, err))
		}
		usage.ResumeAt = &resumeAt
	}
	var diffRange *models.DiffRange
	if row.BaseSha != nil && row.HeadSha != nil {
		diffRange = &models.DiffRange{BaseSHA: *row.BaseSha, HeadSHA: *row.HeadSha}
	}
	return models.Task{
		ID:           models.TaskIDFromUUID(id),
		WorkspaceID:  row.WorkspaceID,
		Title:        row.Title,
		Prompt:       row.Prompt,
		Branch:       row.Branch,
		BaseBranch:   row.BaseBranch,
		WorktreePath: row.WorktreePath,
		Status:       status,
		Priority:     priority,
		Limits: models.ResourceLimits{
			MemoryMB:   limitFromColumn(row.MemoryLimitMb),
			CPUPercent: limitFromColumn(row.CpuLimitPercent),
		},
		Agent:          agentFromRow(row),
		Error:          row.Error,
		UsageLimit:     usage,
		PendingDialogs: dialogs,
		Subagents: models.NewSubagentActivity(
			clampActivity(row.SubagentsActive),
			clampActivity(row.SubagentsWaiting),
		),
		DiffRange: diffRange,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// taskToInsertParams maps the domain task to the insert statement.
func taskToInsertParams(task models.Task) (gen.InsertTaskParams, error) {
	dialogs, err := encodePendingDialogs(task.PendingDialogs)
	if err != nil {
		return gen.InsertTaskParams{}, err
	}
	params := gen.InsertTaskParams{
		ID:               task.ID.String(),
		WorkspaceID:      task.WorkspaceID,
		Title:            task.Title,
		Prompt:           task.Prompt,
		Branch:           task.Branch,
		BaseBranch:       task.BaseBranch,
		WorktreePath:     task.WorktreePath,
		Status:           task.Status.String(),
		Error:            task.Error,
		CreatedAt:        formatTimestamp(task.CreatedAt),
		UpdatedAt:        formatTimestamp(task.UpdatedAt),
		PendingDialogs:   dialogs,
		SubagentsActive:  clampCount(task.Subagents.Active),
		SubagentsWaiting: clampCount(task.Subagents.Waiting),
		Priority:         task.Priority.String(),
		MemoryLimitMb:    limitToColumn(task.Limits.MemoryMB),
		CpuLimitPercent:  limitToColumn(task.Limits.CPUPercent),
		ResumeAttempts:   clampCount(task.UsageLimit.Attempts),
	}
	if task.Agent.Runtime != nil {
		runtimeStr := task.Agent.Runtime.String()
		params.AgentRuntime = &runtimeStr
	}
	params.AgentModelProvider = task.Agent.ModelProvider
	params.AgentModelID = task.Agent.ModelID
	params.AgentThinking = task.Agent.Thinking
	params.AgentProfile = task.Agent.Profile
	if task.UsageLimit.ResumeAt != nil {
		resumeAt := formatTimestamp(*task.UsageLimit.ResumeAt)
		params.ResumeAt = &resumeAt
	}
	if task.DiffRange != nil {
		base := task.DiffRange.BaseSHA
		head := task.DiffRange.HeadSHA
		params.BaseSha = &base
		params.HeadSha = &head
	}
	return params, nil
}

func clampAttempts(value int64) uint32 {
	if value < 0 {
		return 0
	}
	return uint32(min(value, math.MaxUint32))
}

func agentFromRow(row gen.Task) models.TaskAgentOptions {
	var agent models.TaskAgentOptions
	if row.AgentRuntime != nil {
		id, err := models.ParseRuntimeID(*row.AgentRuntime)
		if err == nil {
			agent.Runtime = &id
		}
	}
	agent.ModelProvider = row.AgentModelProvider
	agent.ModelID = row.AgentModelID
	agent.Thinking = row.AgentThinking
	agent.Profile = row.AgentProfile
	return agent
}

func clampActivity(value int64) uint32 {
	return clampAttempts(value)
}
