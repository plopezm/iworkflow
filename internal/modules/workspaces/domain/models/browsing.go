package models

// DirectoryEntry is a folder listed by the directory browser.
type DirectoryEntry struct {
	Name string
	Path string
	// IsGitRepo is true when the folder contains a .git entry.
	IsGitRepo bool
}

// DirectoryListing is the result of listing a folder.
type DirectoryListing struct {
	Path string
	// Parent is empty when Path is the filesystem root.
	Parent  string
	Entries []DirectoryEntry
}

// AgentsFileOption is a stored AGENTS.md that can be installed in a new project.
type AgentsFileOption struct {
	ID          string
	Description string
}

// GitRepositoryInfo is the basic state of the git repository that contains a folder.
type GitRepositoryInfo struct {
	Root string
	// CurrentBranch is the symbolic HEAD branch, even when it is unborn. Empty only when HEAD is detached.
	CurrentBranch string
	// HasCommits is true when at least one commit is reachable from HEAD.
	HasCommits bool
}
