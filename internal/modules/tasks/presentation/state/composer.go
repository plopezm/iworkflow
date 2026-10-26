package state

import "strings"

// ModeStatusKey is the status key under which the runtime reports the agent mode. A runtime without modes never sets
// it, and then the composer hides the mode switch.
const ModeStatusKey = "mode"

// Command names of the mode switch, typed as a whole message.
const (
	planCommand  = "/plan"
	agentCommand = "/agent"
)

// ModeCommand reads a message that switches the mode: exactly /plan or /agent, surrounding whitespace ignored. The
// returned value is the wire name of the mode; the boolean is false for every other message.
func ModeCommand(text string) (string, bool) {
	switch strings.TrimSpace(text) {
	case planCommand:
		return "plan", true
	case agentCommand:
		return "agent", true
	default:
		return "", false
	}
}

// Mode is the mode the transcript reports. The boolean is false when the runtime reported none.
func (t Transcript) Mode() (string, bool) {
	mode, ok := t.Statuses[ModeStatusKey]
	return mode, ok
}

// ComposerView is the message box of the detail page of a task. Scope is the page instance, as on the rest of the page;
// Mode is the agent mode shown as active, empty when the runtime reports none; Notice is the answer of the last send
// or mode change, empty when there is none.
type ComposerView struct {
	Scope  string
	TaskID string
	Mode   string
	Notice string
}

// NewComposerView builds the composer of a task from the mode its transcript reports.
func NewComposerView(scope string, taskID string, transcript Transcript) ComposerView {
	mode, _ := transcript.Mode()
	return ComposerView{Scope: scope, TaskID: taskID, Mode: mode}
}

// HasMode reports whether the runtime reported a mode, which is when the mode switch is shown.
func (v ComposerView) HasMode() bool {
	return v.Mode != ""
}

// ID is the id of the composer element of this page instance. The send answer replaces its content.
func (v ComposerView) ID() string {
	return Scoped("task-composer", v.Scope)
}

// ModeID is the id of the mode switch of this page instance. A mode change replaces only this element.
func (v ComposerView) ModeID() string {
	return Scoped("task-composer-mode", v.Scope)
}

// MessagesPath is the address the composer sends its message to, carrying the page instance.
func (v ComposerView) MessagesPath() string {
	return ScopedPath(TaskPath(v.TaskID)+"/messages", v.Scope)
}

// ModePath is the address the mode switch posts to, carrying the page instance.
func (v ComposerView) ModePath() string {
	return ScopedPath(TaskPath(v.TaskID)+"/mode", v.Scope)
}

// FilesPath is the address that answers the file suggestions of the @ completion.
func (v ComposerView) FilesPath() string {
	return TaskPath(v.TaskID) + "/composer/files"
}
