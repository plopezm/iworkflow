package inbound

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An entry lives while an answer holds the dialog and is removed when the last answer releases it, so answering many
// dialogs leaves no entry behind.
func TestDialogLocksRemoveTheEntryOfEveryDialogOnceReleased(t *testing.T) {
	t.Parallel()
	var locks dialogLocks
	held := dialogKey{task: "t1", dialog: "a"}

	release := locks.lock(held)
	assert.Len(t, locks.locks, 1)
	release()
	assert.Empty(t, locks.locks)

	for _, key := range []dialogKey{{task: "t1", dialog: "a"}, {task: "t1", dialog: "b"}, {task: "t2", dialog: "a"}} {
		locks.lock(key)()
	}
	assert.Empty(t, locks.locks)
}
