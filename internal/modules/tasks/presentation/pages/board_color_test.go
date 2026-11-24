package pages

import "testing"

func TestBoardColorClassMarksOnlyAKnownColor(t *testing.T) {
	for color, want := range map[string]string{"blue": "ws-board ws-blue", "#1a2b3c": "ws-board ws-custom", "": "", "chartreuse": ""} {
		if got := boardColorClass(color); got != want {
			t.Errorf("boardColorClass(%q) = %q, want %q", color, got, want)
		}
	}
}
