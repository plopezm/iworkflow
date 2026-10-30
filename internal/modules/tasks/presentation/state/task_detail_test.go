package state_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestShortBranchDropsTheUUIDSuffixOfAWorktreeBranch(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "pi/fix-login", state.ShortBranch("pi/fix-login-3f2a9c1e-5b7d-4e8a-9c0b-1a2b3c4d5e6f"))
}

func TestShortBranchKeepsShortPlainNames(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "pi/fix-login", state.ShortBranch("pi/fix-login"))
	assert.Equal(t, "main", state.ShortBranch("main"))
}

func TestShortBranchMiddleTruncatesLongNamesToThirtySixCharacters(t *testing.T) {
	t.Parallel()
	long := "pi/" + strings.Repeat("a-very-long-descriptive-name", 3)

	short := state.ShortBranch(long)

	assert.Equal(t, 36, len([]rune(short)))
	assert.Contains(t, short, "…")
	assert.True(t, strings.HasPrefix(short, "pi/"))
	assert.True(t, strings.HasSuffix(short, "name"))
}

func TestShortBranchKeepsAShortHexSuffixThatIsNotAnIdentifier(t *testing.T) {
	t.Parallel()
	// A single hex chunk is an identifier only from 8 digits on, as the Rust version reads it.
	assert.Equal(t, "pi/fix-abc", state.ShortBranch("pi/fix-abc"))
	assert.Equal(t, "pi/fix", state.ShortBranch("pi/fix-deadbeef"), "a single chunk of 8 hex digits is an identifier")
}

func TestResumeClockCutsTheRFC3339TimeToMinutesInUTC(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2026-10-25 14:30", state.ResumeClock("2026-10-25T14:30:00+00:00"))
	assert.Equal(t, "2026-10-25", state.ResumeClock("2026-10-25"), "a value too short to hold a time is kept as it is")
}
