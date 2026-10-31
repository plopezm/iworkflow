package state

import "net/url"

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

// ActivityTrigger is the hx-trigger that refreshes a view of one task. It listens to the page stream (the one the shell
// opens, shared by every view) and reacts to the activity of that task only, after a short pause so a burst of events
// costs one refresh. It also reacts to a (re)connection of the stream, because the events sent while it was down are
// not replayed: the refresh brings the view up to date.
func ActivityTrigger(taskID string) string {
	return activityTrigger(taskID, "")
}

// BodyActivityTrigger is ActivityTrigger for a view that sits outside the element that holds the stream, such as a
// dialog: it listens on the body, where the stream events bubble up to.
func BodyActivityTrigger(taskID string) string {
	return activityTrigger(taskID, " from:body")
}

func activityTrigger(taskID string, source string) string {
	return "sse:activity[event.detail.data=='" + taskID + "']" + source + " delay:300ms, sse:reset" + source + " delay:300ms"
}
