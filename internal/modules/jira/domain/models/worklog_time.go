//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible.
package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

const (
	worklogStep = 5 * time.Minute
	// workDay is what Jira counts as one day (`1d`) in a worklog.
	workDay = 8 * time.Hour
	// maxDescriptionCommits bounds the commit list of a generated description.
	maxDescriptionCommits = 20
	// maxWorklogDuration bounds one worklog; it also keeps the arithmetic far from overflow.
	maxWorklogDuration = 30 * workDay
	// maxAmountDigits bounds each number typed in the duration.
	maxAmountDigits = 5
)

// ProposeWorklogTime is the time to suggest for a worklog: the agent working time not logged yet,
// rounded to the nearest five minutes with a five-minute minimum. Nothing pending proposes zero.
func ProposeWorklogTime(agentRunTime time.Duration, alreadyLogged time.Duration) time.Duration {
	pending := agentRunTime - alreadyLogged
	if pending <= 0 {
		return 0
	}
	rounded := pending.Round(worklogStep)
	return max(rounded, worklogStep)
}

// FormatJiraDuration writes a duration the way Jira does: `1h 30m`, `2h` or `45m`.
func FormatJiraDuration(d time.Duration) string {
	total := int(d / time.Minute)
	hours, minutes := total/60, total%60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// ParseJiraDuration reads a Jira duration made of days, hours and minutes (`1d 2h 30m`, `1h30m`).
// Each unit may appear once and the total must be positive.
func ParseJiraDuration(text string) (time.Duration, error) {
	units := map[rune]time.Duration{'d': workDay, 'h': time.Hour, 'm': time.Minute}
	seen := make(map[rune]bool)
	var total time.Duration
	digits := strings.Builder{}
	for _, char := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case unicode.IsSpace(char):
			if digits.Len() > 0 {
				return 0, jiraerrors.InvalidDuration(text)
			}
		case char >= '0' && char <= '9':
			if digits.Len() >= maxAmountDigits {
				return 0, jiraerrors.InvalidDuration(text)
			}
			digits.WriteRune(char)
		default:
			unit, known := units[char]
			if !known || seen[char] || digits.Len() == 0 {
				return 0, jiraerrors.InvalidDuration(text)
			}
			amount, err := strconv.Atoi(digits.String())
			if err != nil {
				return 0, jiraerrors.InvalidDuration(text)
			}
			seen[char] = true
			total += time.Duration(amount) * unit
			digits.Reset()
		}
	}
	if digits.Len() > 0 || total <= 0 || total > maxWorklogDuration {
		return 0, jiraerrors.InvalidDuration(text)
	}
	return total, nil
}

// WorklogDescription builds the text of a worklog without an LLM: the task title followed by the
// subject of each commit. Commits come as `git log --oneline` lines; the hash is dropped. The list is
// bounded and says how many commits were left out.
func WorklogDescription(taskTitle string, commits []string) string {
	subjects := commitSubjects(commits)
	title := strings.TrimSpace(taskTitle)
	if len(subjects) == 0 {
		return title
	}
	lines := make([]string, 0, maxDescriptionCommits+1)
	for _, subject := range subjects[:min(len(subjects), maxDescriptionCommits)] {
		lines = append(lines, "- "+subject)
	}
	if extra := len(subjects) - maxDescriptionCommits; extra > 0 {
		lines = append(lines, fmt.Sprintf("… y %d commits más", extra))
	}
	return title + "\n\n" + strings.Join(lines, "\n")
}

func commitSubjects(commits []string) []string {
	subjects := make([]string, 0, len(commits))
	for _, line := range commits {
		_, subject, found := strings.Cut(strings.TrimSpace(line), " ")
		if subject = strings.TrimSpace(subject); found && subject != "" {
			subjects = append(subjects, subject)
		}
	}
	return subjects
}
