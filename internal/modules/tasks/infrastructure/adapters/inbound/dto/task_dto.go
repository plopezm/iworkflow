// Package dto holds the client/server shapes of the tasks module. They are plain data with JSON tags
// matching the Rust serde output, and the mapping from and to the domain lives next to them.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package dto

import (
	"time"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// timestampLayout is RFC 3339 with a numeric offset, so UTC is written as "+00:00" like chrono does.
const timestampLayout = "2006-01-02T15:04:05.999999999-07:00"

// TaskActionsDTO says which actions the board may offer for a task.
type TaskActionsDTO struct {
	CanMessage      bool `json:"can_message"`
	CanAbort        bool `json:"can_abort"`
	CanFinish       bool `json:"can_finish"`
	CanResume       bool `json:"can_resume"`
	CanMerge        bool `json:"can_merge"`
	CanDiscard      bool `json:"can_discard"`
	CanReset        bool `json:"can_reset"`
	CanViewDiff     bool `json:"can_view_diff"`
	CanEditFiles    bool `json:"can_edit_files"`
	CanStart        bool `json:"can_start"`
	CanEdit         bool `json:"can_edit"`
	CanDelete       bool `json:"can_delete"`
	CanOpenTerminal bool `json:"can_open_terminal"`
}

// TaskDTO is a task as the board receives it.
type TaskDTO struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	Title         string  `json:"title"`
	Prompt        string  `json:"prompt"`
	Branch        string  `json:"branch"`
	BaseBranch    string  `json:"base_branch"`
	WorktreePath  string  `json:"worktree_path"`
	Status        string  `json:"status"`
	Priority      string  `json:"priority"`
	MemoryLimitMB *uint32 `json:"memory_limit_mb"`
	CPUPercent    *uint32 `json:"cpu_limit_percent"`
	AgentRuntime  *string `json:"agent_runtime,omitempty"`
	AgentModel    *string `json:"agent_model,omitempty"`
	AgentThinking *string `json:"agent_thinking,omitempty"`
	AgentProfile  *string `json:"agent_profile,omitempty"`
	Error         *string `json:"error"`
	// ResumeAt is RFC 3339 in UTC: when the resume after a usage limit is retried.
	ResumeAt       *string        `json:"resume_at"`
	Actions        TaskActionsDTO `json:"actions"`
	Stage          string         `json:"stage"`
	NeedsAttention bool           `json:"needs_attention"`
	PendingDialogs int            `json:"pending_dialogs"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

// TaskFromDomain maps a task to its view as of now (dialogs that expired by now do not count).
func TaskFromDomain(task models.Task, now time.Time) TaskDTO {
	dto := TaskDTO{
		ID:             task.ID.String(),
		WorkspaceID:    task.WorkspaceID,
		Title:          task.Title,
		Prompt:         task.Prompt,
		Branch:         task.Branch,
		BaseBranch:     task.BaseBranch,
		WorktreePath:   task.WorktreePath,
		Status:         task.Status.String(),
		Priority:       task.Priority.String(),
		MemoryLimitMB:  task.Limits.MemoryMB,
		CPUPercent:     task.Limits.CPUPercent,
		AgentRuntime:   agentRuntimeString(task.Agent.Runtime),
		AgentModel:     task.Agent.ModelID,
		AgentThinking:  task.Agent.Thinking,
		AgentProfile:   task.Agent.Profile,
		Error:          task.Error,
		Actions:        actionsFor(task),
		Stage:          StageName(task.StageAt(now)),
		NeedsAttention: task.NeedsAttentionAt(now),
		PendingDialogs: task.PendingDialogCountAt(now),
		CreatedAt:      task.CreatedAt.UTC().Format(timestampLayout),
		UpdatedAt:      task.UpdatedAt.UTC().Format(timestampLayout),
	}
	if task.UsageLimit.ResumeAt != nil {
		resumeAt := task.UsageLimit.ResumeAt.UTC().Format(timestampLayout)
		dto.ResumeAt = &resumeAt
	}
	return dto
}

func agentRuntimeString(r *models.RuntimeID) *string {
	if r == nil {
		return nil
	}
	s := r.String()
	return &s
}

// actionsFor derives the offered actions from the status rules of the domain.
func actionsFor(task models.Task) TaskActionsDTO {
	status := task.Status
	return TaskActionsDTO{
		CanMessage:      status.AcceptsMessages(),
		CanAbort:        status.HasLiveAgent(),
		CanFinish:       status.CanFinish(),
		CanResume:       status.CanResume(),
		CanMerge:        status.CanMerge(),
		CanDiscard:      status.CanDiscard(),
		CanReset:        status.CanReset(),
		CanViewDiff:     task.CanViewDiff(),
		CanEditFiles:    status.HasWorktree(),
		CanStart:        status.CanStart(),
		CanEdit:         status.CanEdit(),
		CanDelete:       status.CanDelete(),
		CanOpenTerminal: status.HasWorktree(),
	}
}

// StageName is the persisted-style name of a board stage.
func StageName(stage models.TaskStage) string {
	switch stage {
	case models.StageBacklog:
		return "backlog"
	case models.StageWorking:
		return "working"
	case models.StageAttention:
		return "attention"
	case models.StageReview:
		return "review"
	default:
		return "done"
	}
}

// ImageDTO is an image attached to a create or update request. Data is base64.
type ImageDTO struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

// CreateTaskDTO is the form of a new task.
type CreateTaskDTO struct {
	WorkspaceID   string     `json:"workspace_id"`
	Title         string     `json:"title"`
	Prompt        string     `json:"prompt"`
	Priority      string     `json:"priority"`
	MemoryLimitMB *uint32    `json:"memory_limit_mb"`
	CPUPercent    *uint32    `json:"cpu_limit_percent"`
	StartNow      bool       `json:"start_now"`
	BaseBranch    *string    `json:"base_branch"`
	Images        []ImageDTO `json:"images"`
}

// ToNewTask converts the form to the domain input. Unknown priority names are a domain error.
func (d CreateTaskDTO) ToNewTask() (models.NewTask, error) {
	priority := models.DefaultTaskPriority
	if d.Priority != "" {
		parsed, err := models.ParseTaskPriority(d.Priority)
		if err != nil {
			return models.NewTask{}, err
		}
		priority = parsed
	}
	return models.NewTask{
		WorkspaceID: d.WorkspaceID,
		Title:       d.Title,
		Prompt:      d.Prompt,
		Priority:    priority,
		Limits:      models.ResourceLimits{MemoryMB: d.MemoryLimitMB, CPUPercent: d.CPUPercent},
		BaseBranch:  d.BaseBranch,
		Images:      imagesToDomain(d.Images),
		StartNow:    d.StartNow,
	}, nil
}

// UpdateTaskDTO is the editable form of a backlog task.
type UpdateTaskDTO struct {
	Title         string     `json:"title"`
	Prompt        string     `json:"prompt"`
	Priority      string     `json:"priority"`
	MemoryLimitMB *uint32    `json:"memory_limit_mb"`
	CPUPercent    *uint32    `json:"cpu_limit_percent"`
	BaseBranch    *string    `json:"base_branch"`
	Images        []ImageDTO `json:"images"`
	// RemoveAttachmentIDs are stored images the task drops.
	RemoveAttachmentIDs []string `json:"remove_attachment_ids"`
}

// ToUpdateInput converts the form to the use case input. An empty priority keeps the default.
func (d UpdateTaskDTO) ToUpdateInput() (application.UpdateInput, error) {
	priority := models.DefaultTaskPriority
	if d.Priority != "" {
		parsed, err := models.ParseTaskPriority(d.Priority)
		if err != nil {
			return application.UpdateInput{}, err
		}
		priority = parsed
	}
	return application.UpdateInput{
		Title:      d.Title,
		Prompt:     d.Prompt,
		Priority:   priority,
		Limits:     models.ResourceLimits{MemoryMB: d.MemoryLimitMB, CPUPercent: d.CPUPercent},
		BaseBranch: d.BaseBranch,
		Images:     imagesToDomain(d.Images),
		RemoveIDs:  d.RemoveAttachmentIDs,
	}, nil
}

// CreatedTaskDTO is the result of creating a task: the task, and the start failure when one was asked and failed.
type CreatedTaskDTO struct {
	Task       TaskDTO `json:"task"`
	StartError *string `json:"start_error"`
}

func imagesToDomain(images []ImageDTO) []models.ImageAttachment {
	converted := make([]models.ImageAttachment, 0, len(images))
	for _, image := range images {
		converted = append(converted, models.ImageAttachment{Name: image.Name, MimeType: image.MimeType, Data: image.Data})
	}
	return converted
}
