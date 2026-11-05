package datahome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	pendingFile   string = "move-pending.json"
	resultFile    string = "move-result.json"
	stagingSuffix string = ".iwf-moving"
	// completeMarker is written inside the staged copy once it is verified and rewritten, so a restart
	// can tell a finished copy from a half one.
	completeMarker string = ".iwf-move-complete"
	trashFolder    string = "worktrees/.trash"
	// progressEvery spaces the copy progress lines; a supervising shell treats them as signs of life.
	progressEvery = 2 * time.Second
)

var (
	// ErrPending is returned when a move is already waiting for the next start.
	ErrPending = errors.New("a move is already pending")
	// ErrDataFolderChanged is returned at start-up when the data folder in use is not the one the move was requested for.
	ErrDataFolderChanged = errors.New("the data folder changed since the move was requested")
	// ErrStoppedForTest is returned when Mover.StopAfter interrupts a move on purpose.
	ErrStoppedForTest = errors.New("move stopped on purpose")
)

// Step names a point of the move, used to simulate a crash in tests.
type Step string

// Steps at which Mover.StopAfter can interrupt a move.
const (
	StepRenamed  Step = "renamed"
	StepSwitched Step = "switched"
)

// Files are the bootstrap files of a move. They live next to the pointer, outside the data folder.
type Files struct {
	Pointer string
	Pending string
	Result  string
}

// FilesFor derives the bootstrap files from the pointer path.
func FilesFor(pointer string) Files {
	dir := filepath.Dir(pointer)
	return Files{Pointer: pointer, Pending: filepath.Join(dir, pendingFile), Result: filepath.Join(dir, resultFile)}
}

// PendingMove is a move waiting for the next start.
type PendingMove struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	RequestedAt time.Time `json:"requestedAt"`
}

// Result is the outcome of the last move, kept for the settings screen.
type Result struct {
	OK         bool      `json:"ok"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	Error      string    `json:"error,omitempty"`
	Warnings   []string  `json:"warnings,omitempty"`
	FinishedAt time.Time `json:"finishedAt"`
}

// Mover requests and performs the move of the data folder.
//
// A request only writes a marker. The copy happens at the next start, before the database or the log
// are opened, so nothing writes to the folder while it is copied. Every step is safe to repeat, and the
// old folder is deleted only after the copy is verified and the pointer names the new folder.
type Mover struct {
	Files Files
	Now   func() time.Time
	Log   func(msg string, args ...any)
	// StopAfter interrupts the move after a step, to test the recovery of a crash. Leave empty in production.
	StopAfter Step
}

// Request validates the move and records it for the next start.
func (m *Mover) Request(from string, to string) error {
	if pending, err := m.Pending(); err != nil {
		return err
	} else if pending != nil {
		return ErrPending
	}
	if err := Validate(from, to); err != nil {
		return err
	}
	if err := writeJSON(m.Files.Pending, PendingMove{From: from, To: to, RequestedAt: m.Now()}); err != nil {
		return fmt.Errorf("record the move: %w", err)
	}
	return nil
}

// Cancel withdraws a pending move. Cancelling when nothing is pending is not an error.
func (m *Mover) Cancel() error {
	if err := os.Remove(m.Files.Pending); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cancel the move: %w", err)
	}
	return nil
}

// Pending returns the move waiting for the next start, or nil.
func (m *Mover) Pending() (*PendingMove, error) {
	var pending PendingMove
	found, err := readJSON(m.Files.Pending, &pending)
	if err != nil || !found {
		return nil, err
	}
	return &pending, nil
}

// LastResult returns the outcome of the last move, or nil when none ran.
func (m *Mover) LastResult() (*Result, error) {
	var result Result
	found, err := readJSON(m.Files.Result, &result)
	if err != nil || !found {
		return nil, err
	}
	return &result, nil
}

// Refuse records a failed move without running it, for a move that must not run now (for example because
// IWF_HOME was set after it was requested and would keep the old folder in use). It drops the pending move.
func (m *Mover) Refuse(reason string) (*Result, error) {
	pending, err := m.Pending()
	if err != nil || pending == nil {
		return nil, err
	}
	result := Result{OK: false, From: pending.From, To: pending.To, Error: reason, FinishedAt: m.Now()}
	if err := writeJSON(m.Files.Result, result); err != nil {
		return &result, fmt.Errorf("record the result of the move: %w", err)
	}
	if err := m.Cancel(); err != nil {
		return &result, err
	}
	return &result, nil
}

// Apply performs the pending move, if any. current is the data folder this process resolved. A failed move
// is reported in the result and leaves the old folder in use; only a cancelled context returns an error,
// and then the move stays pending for the next start.
func (m *Mover) Apply(ctx context.Context, current string) (*Result, error) {
	pending, err := m.Pending()
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return nil, nil //nolint:nilnil // Nothing pending is the normal case, not an error.
	}
	m.Log("data folder move: starting", "from", pending.From, "to", pending.To)
	warnings, runErr := m.run(ctx, *pending, current)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("move interrupted: %w", ctx.Err())
	}
	if errors.Is(runErr, ErrStoppedForTest) {
		return nil, runErr
	}
	result := Result{OK: runErr == nil, From: pending.From, To: pending.To, Warnings: warnings, FinishedAt: m.Now()}
	if runErr != nil {
		result.Error = runErr.Error()
	}
	if err := writeJSON(m.Files.Result, result); err != nil {
		return &result, fmt.Errorf("record the result of the move: %w", err)
	}
	if err := m.Cancel(); err != nil {
		return &result, err
	}
	return &result, nil
}

func (m *Mover) run(ctx context.Context, p PendingMove, current string) ([]string, error) {
	pointer, err := Read(m.Files.Pointer)
	if err != nil {
		return nil, err
	}
	if pointer == p.To && current == p.To {
		return m.finish(p)
	}
	if current != p.From {
		return nil, fmt.Errorf("%w: using %s, requested for %s", ErrDataFolderChanged, current, p.From)
	}
	if !exists(filepath.Join(p.To, completeMarker)) {
		if err := m.stage(ctx, p); err != nil {
			return nil, err
		}
	}
	if m.StopAfter == StepRenamed {
		return nil, ErrStoppedForTest
	}
	if err := m.reconnect(ctx, p); err != nil {
		m.rollback(ctx, p)
		return nil, err
	}
	if err := Write(m.Files.Pointer, p.To); err != nil {
		m.rollback(ctx, p)
		return nil, err
	}
	if m.StopAfter == StepSwitched {
		return nil, ErrStoppedForTest
	}
	return m.finish(p)
}

// stage copies the folder next to its destination, verifies it, rewrites the stored paths and renames it
// into place. Until the rename the destination does not exist, so a failure leaves nothing behind.
func (m *Mover) stage(ctx context.Context, p PendingMove) error {
	if err := Validate(p.From, p.To); err != nil {
		return err
	}
	staging := p.To + stagingSuffix
	if err := os.RemoveAll(staging); err != nil { // ours by name: a previous attempt that was interrupted
		return fmt.Errorf("remove the previous partial copy: %w", err)
	}
	cleanStaging := func() { _ = os.RemoveAll(staging) } // best effort: it is only a partial copy of data that still exists
	if err := Quiesce(ctx, filepath.Join(p.From, DatabaseFile)); err != nil {
		return err
	}
	release, err := LockDatabase(ctx, filepath.Join(p.From, DatabaseFile))
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			m.Log("data folder move: cannot release the database lock", "error", releaseErr)
		}
	}()
	manifest, err := Scan(p.From, func(rel string) bool {
		// The lock keeps the log and shared-memory files of the database open; they are empty and never copied.
		return filepath.ToSlash(rel) == trashFolder || rel == DatabaseFile+"-wal" || rel == DatabaseFile+"-shm"
	})
	if err != nil {
		return err
	}
	if err := CheckSpace(p.To, manifest.TotalSize()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.To), ownerAccess|0o055); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(p.To), err)
	}
	rebased := manifest.Rebased(p.From, p.To)
	m.Log("data folder move: copying", "bytes", manifest.TotalSize())
	err = CopyProgress(ctx, p.From, staging, rebased, progressEvery, func(copied int64, total int64) {
		m.Log("data folder move: copying", "copied_bytes", copied, "total_bytes", total)
	})
	if err != nil {
		cleanStaging()
		return err
	}
	if unhashed := rebased.Unhashed(); len(unhashed) > 0 {
		cleanStaging()
		return fmt.Errorf("%w: no checksum was recorded for %s, so its content cannot be verified", ErrVerify, unhashed[0])
	}
	m.Log("data folder move: verifying")
	if err := Verify(staging, rebased); err != nil {
		cleanStaging()
		return err
	}
	stagedDB := filepath.Join(staging, DatabaseFile)
	if _, err := RewritePaths(ctx, stagedDB, prefixes(p.From, p.To)); err != nil {
		cleanStaging()
		return err
	}
	if err := CompareDatabases(ctx, filepath.Join(p.From, DatabaseFile), stagedDB); err != nil {
		cleanStaging()
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, completeMarker), []byte(p.From+"\n"), 0o600); err != nil {
		cleanStaging()
		return fmt.Errorf("mark the copy as complete: %w", err)
	}
	if err := removeEmptyDir(p.To); err != nil {
		cleanStaging()
		return err
	}
	if err := os.Rename(staging, p.To); err != nil {
		cleanStaging()
		return fmt.Errorf("move the copy into place: %w", err)
	}
	m.Log("data folder move: copy in place")
	return nil
}

// prefixes maps both spellings of the old folder (as given and with links resolved) to the new one.
func prefixes(from string, to string) []Prefix {
	list := []Prefix{{From: from, To: to}}
	resolvedFrom, resolvedTo := resolveExisting(from), resolveExisting(to)
	if resolvedFrom != from {
		list = append(list, Prefix{From: resolvedFrom, To: resolvedTo})
	}
	return list
}

func removeEmptyDir(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace the empty destination: %w", err)
	}
	return nil
}

// reconnect tells every repository where the worktrees live now.
func (m *Mover) reconnect(ctx context.Context, p PendingMove) error {
	worktrees, err := WorktreesOf(ctx, filepath.Join(p.To, DatabaseFile))
	if err != nil {
		return err
	}
	if _, err := RepairWorktrees(ctx, worktrees); err != nil {
		return err
	}
	return nil
}

// rollback undoes a move that failed after the copy was put in place: git is pointed back at the old
// worktrees (which still exist) and the copy is removed. It never touches the old folder.
func (m *Mover) rollback(ctx context.Context, p PendingMove) {
	if !exists(filepath.Join(p.To, completeMarker)) {
		return // not a copy of ours
	}
	worktrees, err := WorktreesOf(ctx, filepath.Join(p.To, DatabaseFile))
	if err == nil {
		for index, item := range worktrees {
			if rel, relErr := filepath.Rel(p.To, item.Path); relErr == nil && !strings.HasPrefix(rel, "..") {
				worktrees[index].Path = filepath.Join(p.From, rel)
			}
		}
		_, err = RepairWorktrees(ctx, worktrees)
	}
	if err != nil {
		m.Log("cannot point git back at the old worktrees", "error", err)
		return // keep the copy: removing it could leave git pointing at nothing
	}
	if err := os.RemoveAll(p.To); err != nil {
		m.Log("cannot remove the failed copy", "path", p.To, "error", err)
	}
}

// finish removes the completion marker and the old folder. The pointer already names the new folder, so
// a problem here is a warning: the move itself succeeded.
func (m *Mover) finish(p PendingMove) ([]string, error) {
	m.Log("data folder move: cleaning the old folder")
	if err := os.Remove(filepath.Join(p.To, completeMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return []string{fmt.Sprintf("cannot remove the completion marker: %v", err)}, nil
	}
	return m.cleanOld(p), nil
}

// cleanOld deletes what the old folder holds that the new one also holds. Anything else stays, and is reported.
func (m *Mover) cleanOld(p PendingMove) []string {
	if _, err := os.Lstat(p.From); err != nil {
		return nil
	}
	if isUnsafeSource(resolveExisting(p.From), resolveExisting(homeOrEmpty())) {
		return []string{fmt.Sprintf("the old folder %s was not deleted because it is too broad", p.From)}
	}
	var warnings []string
	if err := os.RemoveAll(filepath.Join(p.From, filepath.FromSlash(trashFolder))); err != nil {
		warnings = append(warnings, fmt.Sprintf("cannot empty the old trash: %v", err))
	}
	manifest, err := Scan(p.From, nil)
	if err != nil {
		return append(warnings, fmt.Sprintf("cannot list the old folder: %v", err))
	}
	copied := manifest.Only(func(entry Entry) bool { return exists(filepath.Join(p.To, entry.Rel)) })
	if err := Prune(p.From, copied); err != nil {
		warnings = append(warnings, err.Error())
	}
	if exists(p.From) {
		warnings = append(warnings, fmt.Sprintf("the old folder %s still holds files that were not copied, so it was kept", p.From))
	}
	return warnings
}

func homeOrEmpty() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func readJSON(path string, into any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return false, fmt.Errorf("decode %s: %w", path, err)
	}
	return true, nil
}

// NewMover builds the mover of this machine: the bootstrap files sit next to the per-user pointer.
func NewMover(goos string, getenv func(string) string, now func() time.Time, log func(msg string, args ...any)) (*Mover, error) {
	pointer, err := PointerPath(goos, getenv)
	if err != nil {
		return nil, err
	}
	return &Mover{Files: FilesFor(pointer), Now: now, Log: log}, nil
}
