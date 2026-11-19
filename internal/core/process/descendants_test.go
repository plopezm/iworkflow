package process

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubtreeWalksTheTreeBelowTheRootWithoutTheRoot(t *testing.T) {
	tests := []struct {
		name    string
		root    int
		parents map[int]int
		want    []int
	}{
		{
			name:    "no parents map yields nothing",
			root:    10,
			parents: map[int]int{},
			want:    nil,
		},
		{
			name:    "a root without children yields nothing",
			root:    10,
			parents: map[int]int{11: 99, 12: 11},
			want:    nil,
		},
		{
			name:    "direct children",
			root:    10,
			parents: map[int]int{11: 10, 12: 10},
			want:    []int{11, 12},
		},
		{
			name:    "a chain yields every generation",
			root:    10,
			parents: map[int]int{11: 10, 12: 11, 13: 12},
			want:    []int{11, 12, 13},
		},
		{
			name:    "the result is sorted whatever the map order",
			root:    1,
			parents: map[int]int{9: 1, 3: 1, 7: 3, 5: 1, 2: 7},
			want:    []int{2, 3, 5, 7, 9},
		},
		{
			name:    "unrelated branches are left out",
			root:    10,
			parents: map[int]int{11: 10, 20: 2, 21: 20, 30: 3},
			want:    []int{11},
		},
		{
			name:    "the root's own parent is not a descendant",
			root:    10,
			parents: map[int]int{10: 1, 11: 10},
			want:    []int{11},
		},
		{
			name:    "a parent cycle back to the root terminates and excludes the root",
			root:    10,
			parents: map[int]int{11: 10, 12: 11, 10: 12},
			want:    []int{11, 12},
		},
		{
			name:    "a cycle that does not reach the root terminates",
			root:    10,
			parents: map[int]int{11: 10, 20: 21, 21: 20},
			want:    []int{11},
		},
		{
			name:    "a process that is its own parent does not loop",
			root:    10,
			parents: map[int]int{10: 10, 11: 10, 111: 11},
			want:    []int{11, 111},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, subtree(tt.root, tt.parents))
		})
	}
}

func TestDescendantsRejectsAnInvalidRoot(t *testing.T) {
	for _, root := range []int{0, -1} {
		_, err := Descendants(context.Background(), root)
		assert.Error(t, err, "root %d is not a process whose tree can be walked", root)
	}
}

func TestDescendantsOfAPidThatDoesNotExistIsEmpty(t *testing.T) {
	// Pid values this large are beyond what the kernel hands out on Linux and macOS, so no process has them.
	got, err := Descendants(context.Background(), 1<<30)
	require.NoError(t, err)
	assert.Empty(t, got)
}
