package state

import (
	"strings"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// TaskDetailView is what the detail page of one task shows. Scope is the page instance: its element ids and its
// action targets carry it, so two tabs of the same task never share an id in one document. Notice is the answer
// of the last action on the page, empty when there is none. CanMessage says whether the agent takes messages, which
// is when the composer is shown.
type TaskDetailView struct {
	Scope      string
	Task       dto.TaskDTO
	Notice     string
	CanMessage bool
	// Attachments are the images of the task; AttachmentsError says why they could not be read.
	Attachments      []dto.TaskAttachmentDTO
	AttachmentsError string
	// Transcript is the conversation of the task. TranscriptCursor is the last sequence number it holds, and
	// TranscriptError says why the history could not be read.
	Transcript       Transcript
	TranscriptCursor *int64
	TranscriptError  string
}

// branchMaxChars is the longest branch the header shows; a longer one is cut in the middle.
const branchMaxChars = 36

// resumeClockLength is the length of the "YYYY-MM-DDTHH:MM" part of an RFC 3339 time.
const resumeClockLength = 16

// ShortBranch shortens a branch for the header. It drops a trailing identifier (a UUID, or one hex chunk of eight
// digits or more) and cuts what is left to branchMaxChars characters, keeping both ends.
func ShortBranch(branch string) string {
	base := dropIdentifierSuffix(branch)
	chars := []rune(base)
	if len(chars) <= branchMaxChars {
		return base
	}
	keep := branchMaxChars - 1
	head := (keep + 1) / 2
	tail := keep - head
	return string(chars[:head]) + "…" + string(chars[len(chars)-tail:])
}

// dropIdentifierSuffix removes the last one to five dash-separated chunks when they read as an identifier. The
// longest suffix wins, as a UUID has five chunks.
func dropIdentifierSuffix(branch string) string {
	pieces := strings.Split(branch, "-")
	for chunks := 5; chunks >= 1; chunks-- {
		if len(pieces) < chunks+1 {
			continue
		}
		suffix := strings.Join(pieces[len(pieces)-chunks:], "-")
		if isIdentifier(suffix) && (chunks > 1 || len(suffix) >= 8) {
			return strings.Join(pieces[:len(pieces)-chunks], "-")
		}
	}
	return branch
}

// isIdentifier reports whether value is a hex run of eight digits or more, dashes aside.
func isIdentifier(value string) bool {
	digits := 0
	for _, char := range value {
		switch {
		case char == '-':
		case (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F'):
			digits++
		default:
			return false
		}
	}
	return digits >= 8
}

// ResumeClock is the time of an automatic resume, shown to the minute with a space for the "T" separator. A value
// too short to hold a time is returned as it is.
func ResumeClock(at string) string {
	if len(at) < resumeClockLength {
		return at
	}
	return strings.ReplaceAll(at[:resumeClockLength], "T", " ")
}
