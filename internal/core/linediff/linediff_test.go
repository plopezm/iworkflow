package linediff

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineMarks(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		current string
		want    []Mark
	}{
		{"identical text has no marks", "a\nb\nc", "a\nb\nc", []Mark{None, None, None}},
		{"empty texts have one unmarked line", "", "", []Mark{None}},
		{"inserted lines are added", "a\nd", "a\nb\nc\nd", []Mark{None, Added, Added, None}},
		{"changed line is modified", "a\nb\nc", "a\nB\nc", []Mark{None, Modified, None}},
		{"deletion marks the following line", "a\nb\nc", "a\nc", []Mark{None, Deleted}},
		{"deletion at the end marks the last line", "a\nb\nc", "a\nb", []Mark{None, Deleted}},
		{"deletion of the only line marks the single line", "a\nb", "a", []Mark{Deleted}},
		{
			"replaced block with extra lines is modified then added",
			"a\nb\nc\nz", "a\nX\nY\nZ\nz",
			[]Mark{None, Modified, Modified, Added, None},
		},
		{
			"replaced block with fewer lines is all modified",
			"a\nb\nc\nd\nz", "a\nX\nz",
			[]Mark{None, Modified, None},
		},
		{"empty base marks the empty line as modified", "", "a\nb", []Mark{Modified, Added}},
		{"trailing newline adds an empty last line", "a", "a\n", []Mark{None, Added}},
		{"same trailing newline has no marks", "a\n", "a\n", []Mark{None, None}},
		{"removed trailing newline deletes the last line", "a\n", "a", []Mark{Deleted}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LineMarks(tt.base, tt.current))
		})
	}
}

func TestLineMarksAboveLCSLimitMarksTheWholeMiddleAsModified(t *testing.T) {
	marks := LineMarks(numbered("o", 2500), numbered("n", 2500))

	assert.Len(t, marks, 2500)
	for index, mark := range marks {
		assert.Equal(t, Modified, mark, "line %d", index)
	}
}

func numbered(prefix string, count int) string {
	lines := make([]string, count)
	for index := range lines {
		lines[index] = prefix + strconv.Itoa(index)
	}
	return strings.Join(lines, "\n")
}
