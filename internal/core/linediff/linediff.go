// Package linediff computes the per-line change marks shown in the editor margin.
package linediff

import "strings"

// Mark classifies one line of the current text.
type Mark uint8

const (
	// None means the line is unchanged.
	None Mark = iota
	// Added marks a line that does not exist in the base text.
	Added
	// Modified marks a line that replaces one or more base lines.
	Modified
	// Deleted marks a line right after removed base lines (or the last line when the removal is at the end).
	Deleted
)

// maxLCSCells caps the LCS table. Above it the whole middle part is marked as modified.
const maxLCSCells int = 4_000_000

// LineMarks returns one mark per line of current (split on "\n"; there is always at least one line).
func LineMarks(base string, current string) []Mark {
	oldLines := strings.Split(base, "\n")
	newLines := strings.Split(current, "\n")
	marks := make([]Mark, len(newLines))

	prefix := commonPrefix(oldLines, newLines)
	maxSuffix := min(len(oldLines), len(newLines)) - prefix
	suffix := commonSuffix(oldLines, newLines, maxSuffix)
	oldMiddle := oldLines[prefix : len(oldLines)-suffix]
	newMiddle := newLines[prefix : len(newLines)-suffix]
	if len(oldMiddle) == 0 && len(newMiddle) == 0 {
		return marks
	}

	if len(oldMiddle)*len(newMiddle) > maxLCSCells {
		for index := prefix; index < prefix+len(newMiddle); index++ {
			marks[index] = Modified
		}
		return marks
	}

	table := lcsTable(oldMiddle, newMiddle)
	i, j := 0, 0
	for i < len(oldMiddle) || j < len(newMiddle) {
		if i < len(oldMiddle) && j < len(newMiddle) && oldMiddle[i] == newMiddle[j] {
			i++
			j++
			continue
		}
		hunkStart := j
		removed := 0
		for (i < len(oldMiddle) || j < len(newMiddle)) &&
			(i >= len(oldMiddle) || j >= len(newMiddle) || oldMiddle[i] != newMiddle[j]) {
			if j == len(newMiddle) || (i < len(oldMiddle) && table[i+1][j] >= table[i][j+1]) {
				removed++
				i++
			} else {
				j++
			}
		}
		applyHunk(marks, prefix+hunkStart, removed, j-hunkStart)
	}
	return marks
}

func commonPrefix(oldLines []string, newLines []string) int {
	count := 0
	for count < len(oldLines) && count < len(newLines) && oldLines[count] == newLines[count] {
		count++
	}
	return count
}

func commonSuffix(oldLines []string, newLines []string, limit int) int {
	count := 0
	for count < limit &&
		oldLines[len(oldLines)-1-count] == newLines[len(newLines)-1-count] {
		count++
	}
	return count
}

// lcsTable returns table where table[i][j] is the LCS length of oldLines[i:] and newLines[j:].
func lcsTable(oldLines []string, newLines []string) [][]uint32 {
	table := make([][]uint32, len(oldLines)+1)
	for row := range table {
		table[row] = make([]uint32, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table
}

// applyHunk marks a hunk that starts at start in the current text.
func applyHunk(marks []Mark, start int, removed int, added int) {
	if added == 0 {
		if len(marks) > 0 {
			marks[min(start, len(marks)-1)] = Deleted
		}
		return
	}
	for offset := range added {
		if offset < removed {
			marks[start+offset] = Modified
		} else {
			marks[start+offset] = Added
		}
	}
}
