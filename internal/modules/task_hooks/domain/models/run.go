package models

// TaskResult is how the task event ended; post hooks see it as IWF_TASK_RESULT.
type TaskResult string

// Values of TaskResult. A pre hook runs before the result is known and sees TaskResultNone.
const (
	TaskResultNone    TaskResult = ""
	TaskResultOK      TaskResult = "ok"
	TaskResultFailed  TaskResult = "failed"
	TaskResultAborted TaskResult = "aborted"
)

// RunContext describes the task a hook runs for. Its fields feed the argument templates (e.g. {{.Branch}})
// and the IWF_* environment variables.
type RunContext struct {
	TaskID        string
	TaskTitle     string
	TaskStatus    string
	WorkspacePath string
	WorktreePath  string
	Branch        string
	Phase         Phase
	Event         Event
	Result        TaskResult
}

// RunStatus is how a hook run ended.
type RunStatus string

// Values of RunStatus. Failed and TimedOut both count as a failure for the failure policy.
const (
	RunOK       RunStatus = "ok"
	RunFailed   RunStatus = "failed"
	RunTimedOut RunStatus = "timeout"
)

// RunOutcome is the result of running one hook: its status, the exit code of a script (-1 when it did not
// exit) and the captured output for the task log.
type RunOutcome struct {
	Status   RunStatus
	ExitCode int
	Output   string
}

// Failed reports whether the run counts as a failure for the failure policy.
func (o RunOutcome) Failed() bool { return o.Status != RunOK }
