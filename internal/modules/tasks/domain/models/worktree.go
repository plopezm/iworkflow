package models

// TaskDiff is the diff of a task against its base branch.
type TaskDiff struct {
	Stat      string
	Patch     string
	Truncated bool
	Commits   []string
}

// DiffRange is the commit range of a task: the merge base and the tip of its branch.
type DiffRange struct {
	BaseSHA string
	HeadSHA string
}

// MergeOutcome is the result of integrating a branch. A conflict lists the files that clash.
type MergeOutcome struct {
	Conflict bool
	Files    []string
}

// MergedOutcome is the outcome of a clean integration.
func MergedOutcome() MergeOutcome {
	return MergeOutcome{}
}

// ConflictOutcome is the outcome of an integration that stopped on conflicts.
func ConflictOutcome(files []string) MergeOutcome {
	return MergeOutcome{Conflict: true, Files: files}
}
