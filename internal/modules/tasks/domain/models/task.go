//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

const (
	maxTitleLength = 120
	maxSlugLength  = 40
	shortIDLength  = 8
	branchPrefix   = "pi"
)

// TaskID identifies a task. It is a UUID in canonical (lowercase, hyphenated) form.
type TaskID struct {
	value uuid.UUID
}

// NewTaskID generates a random identifier.
func NewTaskID() (TaskID, error) {
	value, err := uuid.NewRandom()
	if err != nil {
		return TaskID{}, err
	}
	return TaskID{value: value}, nil
}

// TaskIDFromUUID wraps an existing UUID, for example one read from the database.
func TaskIDFromUUID(value uuid.UUID) TaskID {
	return TaskID{value: value}
}

// ParseTaskID parses the textual form; any malformed value yields errors.InvalidID.
func ParseTaskID(value string) (TaskID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return TaskID{}, werrors.InvalidID(value)
	}
	return TaskID{value: parsed}, nil
}

// UUID returns the underlying UUID.
func (id TaskID) UUID() uuid.UUID {
	return id.value
}

// String returns the canonical textual form.
func (id TaskID) String() string {
	return id.value.String()
}

// Short returns the prefix used in branch and folder names (first eight hex digits, no dashes).
func (id TaskID) Short() string {
	return hex.EncodeToString(id.value[:])[:shortIDLength]
}

// Task is a task launched on a workspace and executed in its own worktree.
type Task struct {
	ID          TaskID
	WorkspaceID string
	Title       string
	Prompt      string
	Branch      string
	BaseBranch  string
	// WorktreePath is the absolute folder of the worktree; empty while the task is in backlog.
	WorktreePath string
	Status       TaskStatus
	Priority     TaskPriority
	// Limits are the task's own RAM and CPU limits; each defined resource wins over the global one.
	Limits ResourceLimits
	// Agent holds task-specific overrides for agent, model, effort, and profile.
	Agent TaskAgentOptions
	// Error is the failure message shown with the status, if any.
	Error *string
	// UsageLimit is the scheduled resume after a provider usage limit.
	UsageLimit UsageLimitState
	// PendingDialogs are the dialogs the agent opened and that nobody has answered yet.
	PendingDialogs []PendingDialog
	// Subagents is the activity of the subagents started by the main agent.
	Subagents SubagentActivity
	// DiffRange is the final commit range, recorded when the task is integrated (nil if not captured).
	DiffRange *DiffRange
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreatedTask is a task just created. When the start was requested and failed, StartError holds the
// failure. The task still exists in backlog, so the start can be retried without duplicating it.
type CreatedTask struct {
	Task       Task
	StartError error
}

// CanViewDiff reports whether the diff can be read: open tasks except backlog (which has no branch),
// and merged tasks whose commit range was recorded.
func (t Task) CanViewDiff() bool {
	switch t.Status {
	case StatusBacklog, StatusDiscarded:
		return false
	case StatusMerged:
		return t.DiffRange != nil
	default:
		return true
	}
}

// PendingDialogCountAt counts the dialogs that have not expired at now.
func (t Task) PendingDialogCountAt(now time.Time) int {
	count := 0
	for _, dialog := range t.PendingDialogs {
		if !dialog.IsExpiredAt(now) {
			count++
		}
	}
	return count
}

// StageAt is the aggregated view of the status used by the board. Dialogs that expired at now do
// not count as pending.
func (t Task) StageAt(now time.Time) TaskStage {
	switch t.Status {
	case StatusBacklog:
		return StageBacklog
	case StatusMerged, StatusDiscarded:
		return StageDone
	case StatusAwaitingReview:
		return StageReview
	case StatusIdle:
		if t.PendingDialogCountAt(now) == 0 && t.Subagents.IsWorking() {
			return StageWorking
		}
		return StageAttention
	case StatusConflict, StatusFailed, StatusInterrupted, StatusRateLimited:
		return StageAttention
	default: // starting, running, finishing
		if t.PendingDialogCountAt(now) == 0 {
			return StageWorking
		}
		return StageAttention
	}
}

// NeedsAttentionAt reports whether the user should look at the task: attention or review.
func (t Task) NeedsAttentionAt(now time.Time) bool {
	stage := t.StageAt(now)
	return stage == StageAttention || stage == StageReview
}

// NewTask is the input to create a task.
type NewTask struct {
	WorkspaceID string
	Title       string
	Prompt      string
	Priority    TaskPriority
	Limits      ResourceLimits
	// Agent holds optional overrides for agent, model, thinking, and profile.
	Agent TaskAgentOptions
	// BaseBranch is a local or remote branch (for example origin/develop) the worktree starts from.
	// Nil uses the workspace base branch.
	BaseBranch *string
	// Images are attached to the initial prompt.
	Images []ImageAttachment
	// StartNow asks for the task to start (worktree and agent) right after it is created.
	StartNow bool
	// RelatedProjectIDs are the workspaces (other projects) the task relates to, in the order the user chose.
	RelatedProjectIDs []string
}

// Validated normalizes the input (trimmed texts, derived title) and checks its invariants.
func (n NewTask) Validated() (NewTask, error) {
	prompt := strings.TrimSpace(n.Prompt)
	if prompt == "" {
		return NewTask{}, werrors.EmptyPrompt()
	}
	title := strings.TrimSpace(n.Title)
	if title == "" {
		title = deriveTitle(prompt)
	}
	if utf8.RuneCountInString(title) > maxTitleLength {
		return NewTask{}, werrors.InvalidTitle()
	}
	limits, err := n.Limits.Validated()
	if err != nil {
		return NewTask{}, err
	}
	images, err := ValidateAllImages(n.Images)
	if err != nil {
		return NewTask{}, err
	}
	agent, err := n.Agent.Validated()
	if err != nil {
		return NewTask{}, err
	}
	related, err := validateRelatedProjects(n.WorkspaceID, n.RelatedProjectIDs)
	if err != nil {
		return NewTask{}, err
	}
	var baseBranch *string
	if n.BaseBranch != nil {
		trimmed := strings.TrimSpace(*n.BaseBranch)
		if trimmed != "" {
			baseBranch = &trimmed
		}
	}
	priority := n.Priority
	if priority == "" {
		// An input without a priority means the default, as an omitted field does in the Rust form.
		priority = DefaultTaskPriority
	}
	return NewTask{
		WorkspaceID:       strings.TrimSpace(n.WorkspaceID),
		Title:             title,
		Prompt:            prompt,
		Priority:          priority,
		Limits:            limits,
		Agent:             agent,
		BaseBranch:        baseBranch,
		Images:            images,
		StartNow:          n.StartNow,
		RelatedProjectIDs: related,
	}, nil
}

// validateRelatedProjects trims the workspace ids, drops the blank ones and the task's own workspace, rejects
// anything that is not a UUID and removes duplicates, keeping the order of the first occurrence.
func validateRelatedProjects(ownWorkspaceID string, workspaceIDs []string) ([]string, error) {
	own := strings.TrimSpace(ownWorkspaceID)
	var related []string
	seen := make(map[string]struct{}, len(workspaceIDs))
	for _, raw := range workspaceIDs {
		id := strings.TrimSpace(raw)
		if id == "" || id == own {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			return nil, werrors.InvalidRelatedProject(id)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		related = append(related, id)
	}
	return related, nil
}

// deriveTitle is the first line of the prompt, trimmed and cut to the title limit.
func deriveTitle(prompt string) string {
	firstLine := strings.TrimSpace(strings.SplitN(prompt, "\n", 2)[0])
	runes := []rune(firstLine)
	if len(runes) > maxTitleLength {
		runes = runes[:maxTitleLength]
	}
	return string(runes)
}

// Slugify returns an ASCII lowercase slug apt for branches and folders. Accented Latin letters are
// folded, every other run of characters becomes one dash, and the result is bounded.
func Slugify(text string) string {
	var slug strings.Builder
	lastDash := true
	for _, character := range text {
		mapped, ok := slugRune(character)
		if ok {
			slug.WriteRune(mapped)
			lastDash = false
		} else if !lastDash {
			slug.WriteByte('-')
			lastDash = true
		}
		if slug.Len() >= maxSlugLength {
			break
		}
	}
	trimmed := strings.Trim(slug.String(), "-")
	if trimmed == "" {
		return "task"
	}
	return trimmed
}

func slugRune(character rune) (rune, bool) {
	switch {
	case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
		return character, true
	case character >= 'A' && character <= 'Z':
		return character + ('a' - 'A'), true
	}
	switch character {
	case 'á', 'à', 'ä', 'â', 'Á':
		return 'a', true
	case 'é', 'è', 'ë', 'ê', 'É':
		return 'e', true
	case 'í', 'ì', 'ï', 'î', 'Í':
		return 'i', true
	case 'ó', 'ò', 'ö', 'ô', 'Ó':
		return 'o', true
	case 'ú', 'ù', 'ü', 'û', 'Ú':
		return 'u', true
	case 'ñ', 'Ñ':
		return 'n', true
	}
	return 0, false
}

// BranchName returns the branch of a task: pi/<slug>-<unique>.
func BranchName(title string, unique uuid.UUID) string {
	return branchPrefix + "/" + Slugify(title) + "-" + unique.String()
}

// IsTaskBranch reports whether name is a branch created by iworkflow for a task (pi/...).
func IsTaskBranch(name string) bool {
	rest, found := strings.CutPrefix(name, branchPrefix)
	return found && strings.HasPrefix(rest, "/")
}

// WorktreeDirName returns the worktree folder name: <slug>-<unique>.
func WorktreeDirName(title string, unique uuid.UUID) string {
	return Slugify(title) + "-" + unique.String()
}

// WorkspaceDirName returns the workspace folder under the worktree root: <slug>-<workspace id>.
func WorkspaceDirName(name string, workspaceID string) string {
	return Slugify(name) + "-" + workspaceID
}
