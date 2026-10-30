//go:build unix

package pty

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStatHandlesCommWithSpacesAndParentheses(t *testing.T) {
	parsed, ok := parseStat(42, "42 (we ird) name) S 7 42 40 0 -1")

	require.True(t, ok)
	require.Equal(t, procEntry{pid: 42, ppid: 7, session: 40}, parsed)
}

func TestParseStatRejectsGarbage(t *testing.T) {
	_, ok := parseStat(1, "garbage")

	require.False(t, ok)
}

func TestMembersIncludeSessionOrphansAndSetsidDescendantsButNotRoot(t *testing.T) {
	procs := []procEntry{
		{pid: 100, ppid: 1, session: 100},   // shell (root)
		{pid: 101, ppid: 100, session: 100}, // direct child
		{pid: 102, ppid: 1, session: 100},   // orphan of the same session
		{pid: 103, ppid: 101, session: 103}, // grandchild that called setsid
		{pid: 104, ppid: 103, session: 103}, // great-grandchild
		{pid: 200, ppid: 1, session: 200},   // unrelated
	}

	require.Equal(t, []int{101, 102, 103, 104}, members(100, procs))
}

func TestMembersOfAnUnknownSessionAreEmpty(t *testing.T) {
	procs := []procEntry{{pid: 5, ppid: 1, session: 5}}

	require.Empty(t, members(100, procs))
}
