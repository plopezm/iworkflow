package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithinMatchesTheRootAndItsDescendantsOnly(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "wt", "task")
	cases := map[string]struct {
		path string
		want bool
	}{
		"the root itself":       {root, true},
		"a file inside":         {filepath.Join(root, "src", "main.go"), true},
		"a sibling with prefix": {root + "-other", false},
		"the parent":            {filepath.Dir(root), false},
		"empty":                 {"", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, within(root, tc.path))
		})
	}
}

func TestParseLsofReturnsEachPidOnceAndIgnoresOtherFields(t *testing.T) {
	out := "p100\nfcwd\nn/wt/task\nf3\nn/wt/task/a.txt\np200\nn/wt/task/b\np100\nn/wt/task/c\n"

	users := parseLsof(out)

	require.Len(t, users, 2)
	assert.Equal(t, 100, users[0].PID)
	assert.Equal(t, 200, users[1].PID)
	assert.Empty(t, parseLsof(""))
}

func TestParseLsofMapsEachDescriptorToItsReason(t *testing.T) {
	cases := map[string]struct {
		fd   string
		want Reason
	}{
		"working directory": {"cwd", ReasonWorkingDirectory},
		"executable text":   {"txt", ReasonExecutable},
		"numbered fd":       {"3r", ReasonOpenFile},
		"memory map":        {"mem", ReasonOpenFile},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			users := parseLsof("p7\ncsleep\nf" + tc.fd + "\nn/wt/task/x\n")

			require.Len(t, users, 1)
			assert.Equal(t, tc.want, users[0].Reason)
			assert.Equal(t, "/wt/task/x", users[0].Path)
		})
	}
}

func TestParseLsofKeepsTheStrongestTieAndTheCommandName(t *testing.T) {
	out := "p10\nczsh\nf3\nn/wt/task/log\nftxt\nn/wt/task/bin/tool\nfcwd\nn/wt/task\n" +
		"p20\ncnode\nf4\nn/wt/task/x\nftxt\nn/wt/task/y\n"

	users := parseLsof(out)

	require.Len(t, users, 2)
	assert.Equal(t, User{PID: 10, Name: "zsh", Path: "/wt/task", Reason: ReasonWorkingDirectory}, users[0])
	assert.Equal(t, User{PID: 20, Name: "node", Path: "/wt/task/y", Reason: ReasonExecutable}, users[1])
}

func TestParseLsofLeavesStartedEmptyAndSkipsMalformedPids(t *testing.T) {
	users := parseLsof("pnot-a-pid\nfcwd\nn/wt/task\np30\ncsh\nfcwd\nn/wt/task\n")

	require.Len(t, users, 1)
	assert.Equal(t, 30, users[0].PID)
	assert.Empty(t, users[0].Started)
}

func TestProtectedPIDsKeepTheCallerAlive(t *testing.T) {
	assert.Contains(t, protectedPIDs(), os.Getpid())
}

func TestWithinFoldComparesCaseOnlyWhenAsked(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "Wt", "Task")
	path := filepath.Join(string(filepath.Separator), "wt", "task", "a.go")

	assert.False(t, withinFold(root, path, false))
	assert.True(t, withinFold(root, path, true))
}
