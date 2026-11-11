package models

import "time"

// runClock accumulates the working time of an agent while walking the events of a task in order.
type runClock struct {
	total       time.Duration
	runStart    *time.Time
	dialogStart *time.Time
	paused      time.Duration
}

func (c *runClock) open(at time.Time) {
	if c.runStart == nil {
		c.runStart = &at
		c.dialogStart = nil
		c.paused = 0
	}
}

func (c *runClock) pause(at time.Time) {
	if c.runStart != nil && c.dialogStart == nil {
		c.dialogStart = &at
	}
}

func (c *runClock) resume(at time.Time) {
	if c.dialogStart != nil {
		c.paused += nonNegative(at.Sub(*c.dialogStart))
		c.dialogStart = nil
	}
}

func (c *runClock) close(at time.Time) {
	if c.runStart == nil {
		return
	}
	c.resume(at)
	c.total += nonNegative(at.Sub(*c.runStart) - c.paused)
	c.runStart = nil
	c.paused = 0
}

func nonNegative(d time.Duration) time.Duration {
	return max(d, 0)
}

// AgentRunTime is how long the agent actually worked on a task: the time between a run starting and
// settling, leaving out the time a dialog waited for the user. Time between turns is never counted.
// A run still open at the end is counted until now.
//
// events must be in the order they were stored; the function is pure, so the caller injects now.
func AgentRunTime(events []TaskEvent, now time.Time) time.Duration {
	var clock runClock
	for _, event := range events {
		switch kind := event.Kind.(type) {
		case AgentEventKind:
			clock.apply(kind.Event, event.CreatedAt)
		case StatusEventKind:
			if kind.Status == StatusStarting || kind.Status == StatusRunning {
				clock.open(event.CreatedAt)
			} else {
				clock.close(event.CreatedAt)
			}
		}
	}
	clock.close(now)
	return clock.total
}

func (c *runClock) apply(event AgentEvent, at time.Time) {
	switch event.(type) {
	case RunStarted:
		c.open(at)
	case RunSettled, ProcessExited, UsageLimitReached:
		c.close(at)
	case DialogRequested:
		c.pause(at)
	case DialogResolved:
		c.resume(at)
	}
}
