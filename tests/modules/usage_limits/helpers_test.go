package usagelimits_test

import "time"

// stoppedClock is a ClockPort stopped at a moment.
type stoppedClock struct{ now time.Time }

func (c stoppedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
