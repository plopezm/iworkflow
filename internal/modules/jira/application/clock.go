// Package application holds the use cases of the jira module. Each use case is a struct with one
// Execute method; they depend on the ports of the domain, never on an adapter.
package application

import "time"

// Clock returns the current instant. Use cases receive it so the tests control time.
type Clock func() time.Time

// SystemClock is the Clock of the running process.
func SystemClock() time.Time {
	return time.Now()
}
