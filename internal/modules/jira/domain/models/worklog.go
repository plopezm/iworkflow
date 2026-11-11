package models

import "time"

// Worklog is the time a user logs on an issue: when the work started, how long it took and what was done.
type Worklog struct {
	started          time.Time
	timeSpentSeconds int
	comment          string
}

// NewWorklog builds a worklog entry to send to Jira.
func NewWorklog(started time.Time, timeSpentSeconds int, comment string) Worklog {
	return Worklog{started: started, timeSpentSeconds: timeSpentSeconds, comment: comment}
}

// Started is the instant the logged work began.
func (w Worklog) Started() time.Time { return w.started }

// TimeSpentSeconds is the logged duration in seconds.
func (w Worklog) TimeSpentSeconds() int { return w.timeSpentSeconds }

// Comment is the description of the work done.
func (w Worklog) Comment() string { return w.comment }
