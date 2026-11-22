package components

import "strconv"

// lineID is the stable id of the log line at position i (newest first), so a morph reuses elements.
func lineID(i int) string {
	return "log-line-" + strconv.Itoa(i)
}
