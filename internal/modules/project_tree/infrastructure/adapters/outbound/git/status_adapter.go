// Package git reads and changes the state of a project through the git CLI, using internal/core/gitx.
// Paths handed to git are validated first and passed as literal pathspecs.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"strings"
	"unicode/utf8"

	"iworkflow/internal/core/gitx"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// StatusAdapter reads and changes git state with the git CLI.
type StatusAdapter struct{}

var _ outbound.GitStatusPort = (*StatusAdapter)(nil)

// NewStatusAdapter builds the adapter.
func NewStatusAdapter() *StatusAdapter {
	return &StatusAdapter{}
}

// Changes returns the paths under root that differ from HEAD, relative to root. It is empty when
// root is not inside a git repository.
func (a *StatusAdapter) Changes(ctx context.Context, root string) ([]models.GitChange, error) {
	prefix, err := runGit(ctx, root, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	if !prefix.Success {
		return []models.GitChange{}, nil
	}
	status, err := runGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if !status.Success {
		return []models.GitChange{}, nil
	}
	return parseStatus(status.Stdout, strings.TrimSpace(prefix.Stdout)), nil
}

// BaseContent returns the HEAD version of relative, or nil when git does not know the file.
func (a *StatusAdapter) BaseContent(ctx context.Context, root string, relative string) (*string, error) {
	if err := models.ValidateRelative(relative); err != nil {
		return nil, err
	}
	shown, err := runGit(ctx, root, "show", "HEAD:./"+relative)
	if err != nil {
		return nil, err
	}
	if !shown.Success {
		return nil, nil
	}
	content := shown.Stdout
	return &content, nil
}

// Stage adds paths to the index; no paths means the whole project.
func (a *StatusAdapter) Stage(ctx context.Context, root string, paths []string) error {
	args, err := pathspecCommand([]string{"add", "-A"}, paths)
	if err != nil {
		return err
	}
	return runIndexCommand(ctx, root, args)
}

// Unstage removes paths from the index and keeps the working tree; no paths means the whole project.
func (a *StatusAdapter) Unstage(ctx context.Context, root string, paths []string) error {
	args, err := pathspecCommand([]string{"reset", "-q"}, paths)
	if err != nil {
		return err
	}
	return runIndexCommand(ctx, root, args)
}

// Discard reverts paths to HEAD and deletes the ones HEAD does not know (untracked, newly added or
// renamed files). An empty list is invalid: there is no "discard everything" default.
func (a *StatusAdapter) Discard(ctx context.Context, root string, paths []string) error {
	if len(paths) == 0 {
		return treeerrors.InvalidPath("")
	}
	for _, path := range paths {
		if path == "" {
			return treeerrors.InvalidPath(path)
		}
		if err := models.ValidateRelative(path); err != nil {
			return err
		}
	}
	changes, err := a.Changes(ctx, root)
	if err != nil {
		return err
	}
	kinds := make(map[string]models.GitChangeKind, len(changes))
	for _, change := range changes {
		kinds[change.Path] = change.Kind
	}
	var restore, remove []string
	for _, path := range paths {
		switch kinds[path] {
		case models.GitChangeUntracked, models.GitChangeAdded:
			remove = append(remove, path)
		case models.GitChangeRenamed:
			remove = append(remove, path)
			origin, err := a.renameOrigin(ctx, root, path)
			if err != nil {
				return err
			}
			if origin != "" {
				restore = append(restore, origin)
			}
		case models.GitChangeModified, models.GitChangeDeleted, models.GitChangeConflicted:
			restore = append(restore, path)
		}
	}
	if len(restore) > 0 {
		args, err := pathspecCommand([]string{"restore", "--source=HEAD", "--staged", "--worktree"}, restore)
		if err != nil {
			return err
		}
		if err := runIndexCommand(ctx, root, args); err != nil {
			return err
		}
	}
	return a.removeNew(ctx, root, remove, kinds)
}

// renameOrigin returns the path a staged rename came from (relative to root), or "" when git
// reports none. Discarding the new name must also bring the old file back.
func (a *StatusAdapter) renameOrigin(ctx context.Context, root string, renamed string) (string, error) {
	diff, err := runGit(ctx, root, "diff", "--cached", "--relative", "-M", "--name-status", "-z", "HEAD")
	if err != nil {
		return "", err
	}
	if !diff.Success {
		return "", treeerrors.IO(diff.Stderr)
	}
	fields := nonEmptyFields(diff.Stdout)
	for i := 0; i+2 < len(fields); i++ {
		if strings.HasPrefix(fields[i], "R") && fields[i+2] == renamed {
			return fields[i+1], nil
		}
	}
	return "", nil
}

// removeNew deletes files HEAD does not know. Tracked-in-index ones go through `git rm`, which also
// clears the index entry; plain untracked ones through `git clean`.
func (a *StatusAdapter) removeNew(ctx context.Context, root string, paths []string, kinds map[string]models.GitChangeKind) error {
	var indexed, untracked []string
	for _, path := range paths {
		if kinds[path] == models.GitChangeUntracked {
			untracked = append(untracked, path)
		} else {
			indexed = append(indexed, path)
		}
	}
	if len(indexed) > 0 {
		args, err := pathspecCommand([]string{"rm", "-f", "-q"}, indexed)
		if err != nil {
			return err
		}
		if err := runIndexCommand(ctx, root, args); err != nil {
			return err
		}
	}
	if len(untracked) > 0 {
		args, err := pathspecCommand([]string{"clean", "-f", "-q"}, untracked)
		if err != nil {
			return err
		}
		return runIndexCommand(ctx, root, args)
	}
	return nil
}

// Commit records the index with message. It fails when nothing is staged.
func (a *StatusAdapter) Commit(ctx context.Context, root string, message string) error {
	output, err := runGit(ctx, root, "commit", "-q", "-m", message)
	if err != nil {
		return err
	}
	if output.Success {
		return nil
	}
	return treeerrors.CommitFailed(commitFailureReason(output.Stderr, output.Stdout))
}

// runGit runs git in dir. Only a failure to start git is an error; a non-zero exit is reported in
// the returned output.
func runGit(ctx context.Context, dir string, args ...string) (gitx.Output, error) {
	output, err := gitx.RunRaw(ctx, dir, args...)
	if err != nil {
		return gitx.Output{}, treeerrors.IO(err.Error())
	}
	return output, nil
}

// runIndexCommand runs an index command that must succeed; its stderr is the error detail.
func runIndexCommand(ctx context.Context, root string, args []string) error {
	output, err := runGit(ctx, root, args...)
	if err != nil {
		return err
	}
	if output.Success {
		return nil
	}
	return treeerrors.IO(output.Stderr)
}

// commitFailureReason says why git commit failed (pure). Git reports "nothing to commit" on stdout
// and other failures on stderr, so the first non-empty text wins.
func commitFailureReason(stderr string, stdout string) string {
	for _, text := range []string{stderr, stdout} {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return "git commit failed without output"
}

// pathspecCommand builds `git --literal-pathspecs <command> -- <paths>`; no paths means the whole
// project (`.`). Literal pathspecs stop names such as `:(exclude)x` or `*.rs` from being read as
// git magic.
func pathspecCommand(command []string, paths []string) ([]string, error) {
	args := make([]string, 0, len(command)+len(paths)+3)
	args = append(args, "--literal-pathspecs")
	args = append(args, command...)
	args = append(args, "--")
	if len(paths) == 0 {
		return append(args, "."), nil
	}
	for _, path := range paths {
		if path == "" {
			return nil, treeerrors.InvalidPath(path)
		}
		if err := models.ValidateRelative(path); err != nil {
			return nil, err
		}
		args = append(args, path)
	}
	return args, nil
}

// parseStatus parses `git status --porcelain=v1 -z` output into the changes under prefix (pure).
// prefix is the project root inside the repository ("" or "sub/dir/"): paths outside it are
// dropped and the prefix is stripped from the rest.
func parseStatus(output string, prefix string) []models.GitChange {
	fields := nonEmptyFields(output)
	changes := make([]models.GitChange, 0, len(fields))
	for position := 0; position < len(fields); position++ {
		field := fields[position]
		indexColumn, rest, ok := nextRune(field)
		if !ok {
			continue
		}
		worktreeColumn, _, ok := nextRune(rest)
		if !ok {
			continue
		}
		entryPath := pathOf(field)
		// A rename or copy carries its original path as the next field, which is skipped.
		if isRenameOrCopy(indexColumn) || isRenameOrCopy(worktreeColumn) {
			position++
		}
		kind, ok := kindOf(indexColumn, worktreeColumn)
		if !ok {
			continue
		}
		relative, inside := strings.CutPrefix(entryPath, prefix)
		if !inside {
			continue
		}
		changes = append(changes, models.GitChange{
			Path:     strings.TrimRight(relative, "/"),
			Kind:     kind,
			Staged:   indexColumn != ' ' && indexColumn != '?' && indexColumn != '!',
			Unstaged: worktreeColumn != ' ',
		})
	}
	return changes
}

// kindOf maps the two status columns to a change kind (pure). It reports false for ignored and
// clean entries, which are never listed.
func kindOf(indexColumn rune, worktreeColumn rune) (models.GitChangeKind, bool) {
	switch {
	case indexColumn == '?' && worktreeColumn == '?':
		return models.GitChangeUntracked, true
	case (indexColumn == '!' && worktreeColumn == '!') || (indexColumn == ' ' && worktreeColumn == ' '):
		return 0, false
	case (indexColumn == 'D' && worktreeColumn == 'D') || (indexColumn == 'A' && worktreeColumn == 'A') ||
		indexColumn == 'U' || worktreeColumn == 'U':
		return models.GitChangeConflicted, true
	case indexColumn == 'A':
		return models.GitChangeAdded, true
	case isRenameOrCopy(indexColumn) || isRenameOrCopy(worktreeColumn):
		return models.GitChangeRenamed, true
	case indexColumn == 'D' || worktreeColumn == 'D':
		return models.GitChangeDeleted, true
	default:
		return models.GitChangeModified, true
	}
}

// isRenameOrCopy reports whether a status column marks a rename or a copy.
func isRenameOrCopy(column rune) bool {
	return column == 'R' || column == 'C'
}

// nonEmptyFields splits NUL-separated git output and drops the empty fields.
func nonEmptyFields(output string) []string {
	fields := make([]string, 0)
	for _, field := range strings.Split(output, "\x00") {
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

// nextRune returns the first character of text and the rest of it. It reports false when text is
// empty.
func nextRune(text string) (rune, string, bool) {
	if text == "" {
		return 0, "", false
	}
	first, size := utf8.DecodeRuneInString(text)
	return first, text[size:], true
}

// pathOf returns the path that follows the two status columns and the space. Like the original
// slicing it yields "" when the field is too short or the cut would split a character.
func pathOf(field string) string {
	if len(field) <= 3 || !utf8.RuneStart(field[3]) {
		return ""
	}
	return field[3:]
}
