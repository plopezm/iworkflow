package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestProposeWorklogTimeRoundsToFiveMinutesWithAFiveMinuteMinimum(t *testing.T) {
	tests := []struct {
		name   string
		run    time.Duration
		logged time.Duration
		want   time.Duration
	}{
		{"zero work proposes nothing", 0, 0, 0},
		{"tiny work rounds up to the minimum", 40 * time.Second, 0, 5 * time.Minute},
		{"rounds down to the nearest five", 12*time.Minute + 20*time.Second, 0, 10 * time.Minute},
		{"rounds up to the nearest five", 13 * time.Minute, 0, 15 * time.Minute},
		{"only the unlogged time is proposed", 95 * time.Minute, 60 * time.Minute, 35 * time.Minute},
		{"everything logged proposes nothing", 60 * time.Minute, 60 * time.Minute, 0},
		{"more logged than worked proposes nothing", 10 * time.Minute, 60 * time.Minute, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, models.ProposeWorklogTime(tt.run, tt.logged))
		})
	}
}

func TestFormatJiraDuration(t *testing.T) {
	assert.Equal(t, "1h 30m", models.FormatJiraDuration(90*time.Minute))
	assert.Equal(t, "2h", models.FormatJiraDuration(2*time.Hour))
	assert.Equal(t, "45m", models.FormatJiraDuration(45*time.Minute))
	assert.Equal(t, "0m", models.FormatJiraDuration(0))
}

func TestParseJiraDuration(t *testing.T) {
	valid := map[string]time.Duration{
		"1h 30m":  90 * time.Minute,
		"1h30m":   90 * time.Minute,
		"2h":      2 * time.Hour,
		" 45m ":   45 * time.Minute,
		"1d":      8 * time.Hour,
		"1d 2h":   10 * time.Hour,
		"1H 5M":   65 * time.Minute,
		"1h 120m": 3 * time.Hour,
	}
	for input, want := range valid {
		got, err := models.ParseJiraDuration(input)
		require.NoError(t, err, input)
		assert.Equal(t, want, got, input)
	}
	for _, input := range []string{"", "  ", "abc", "1x", "h", "1.5h", "-1h", "0m", "1h 1h"} {
		_, err := models.ParseJiraDuration(input)
		require.ErrorIs(t, err, jiraerrors.InvalidDuration(""), input)
	}
}

func TestWorklogDescriptionListsTheTaskAndItsCommitSubjectsWithoutHashes(t *testing.T) {
	got := models.WorklogDescription("PROJ-1 Fix login", []string{"a1b2c3d feat: add form", "e4f5a6b fix: trim input"})

	assert.Equal(t, "PROJ-1 Fix login\n\n- feat: add form\n- fix: trim input", got)
}

func TestWorklogDescriptionWithoutCommitsIsTheTaskTitle(t *testing.T) {
	assert.Equal(t, "PROJ-1 Fix login", models.WorklogDescription("PROJ-1 Fix login", nil))
	assert.Equal(t, "PROJ-1 Fix login", models.WorklogDescription("  PROJ-1 Fix login ", []string{"  ", "abc1234"}))
}

func TestWorklogDescriptionBoundsTheCommitList(t *testing.T) {
	var commits []string
	for range 25 {
		commits = append(commits, "abc1234 chore: step")
	}

	got := models.WorklogDescription("T", commits)

	assert.Contains(t, got, "… y 5 commits más")
	assert.Equal(t, 20, countLines(got, "- chore: step"))
}

func countLines(text string, line string) int {
	count := 0
	for _, current := range splitLines(text) {
		if current == line {
			count++
		}
	}
	return count
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for index := range len(text) {
		if text[index] == '\n' {
			lines = append(lines, text[start:index])
			start = index + 1
		}
	}
	return append(lines, text[start:])
}
