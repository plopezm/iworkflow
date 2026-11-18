package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, []int{100, 200}, parseLsof(out))
	assert.Empty(t, parseLsof(""))
}

func TestProtectedPIDsKeepTheCallerAlive(t *testing.T) {
	assert.Contains(t, protectedPIDs(), os.Getpid())
}
