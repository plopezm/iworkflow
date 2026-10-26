package state

import (
	"net/url"
	"strconv"
)

// TranscriptPath is the address that answers the body of the transcript of a task. The page asks for it again each
// time the task stream reports an event.
func TranscriptPath(taskID string) string {
	return "/tasks/" + taskID + "/transcript"
}

// DialogAnswerPath is the address a dialog of the transcript posts its answer to. The dialog id comes from the agent,
// so it is escaped as a path segment.
func DialogAnswerPath(taskID string, dialogID string) string {
	return "/tasks/" + taskID + "/dialogs/" + url.PathEscape(dialogID) + "/answer"
}

// TaskEventsPath is the task stream the transcript listens to. With a cursor the stream starts after the last
// sequence number the page already shows, so the history is not sent twice.
func TaskEventsPath(taskID string, cursor *int64) string {
	path := "/api/tasks/" + taskID + "/events"
	if cursor == nil {
		return path
	}
	return path + "?after=" + strconv.FormatInt(*cursor, 10)
}
