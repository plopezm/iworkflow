package models

import (
	"strings"
	"time"
)

// probeBaseDelay is the first wait between attempts when the provider does not say when the limit renews.
const probeBaseDelay = 5 * time.Minute

// probeMaxDelay caps the wait between attempts (the backoff doubles up to here).
const probeMaxDelay = time.Hour

// resetMargin is added after the renewal instant the provider reports, so the resume does not land
// just before it.
const resetMargin = time.Minute

// maxBackoffExponent bounds the backoff exponent (2^6 * 5 min already exceeds the cap).
const maxBackoffExponent = 6

// Bounds of the instants chrono accepts (years -262144 to 262143), in epoch seconds.
const (
	minEpochSecs int64 = -8_334_601_228_800
	maxEpochSecs int64 = 8_210_266_876_799
)

// limitMarkers are lowercase fragments of error messages with which providers report that the quota,
// credit or subscription window ran out.
var limitMarkers = [...]string{
	"usage limit",
	"out of extra usage",
	"rate limit",
	"rate_limit",
	"too many requests",
	"insufficient_quota",
	"exceeded your current quota",
	"credit balance is too low",
	"upgrade your plan",
	"resource_exhausted",
	// Hard limits that pi does not retry.
	"available balance",
	"quota exceeded",
	"out of budget",
	"subscription_sharing_usage_limit",
}

// LooksLikeUsageLimit reports whether text (an error message of the agent) describes an exhausted
// usage limit: "You've hit your session limit · resets 3:45pm", "usage limit reached", a 429, etc.
func LooksLikeUsageLimit(text string) bool {
	lowered := strings.ToLower(text)
	if strings.Contains(lowered, "hit your") && strings.Contains(lowered, "limit") {
		return true
	}
	for _, marker := range limitMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// FromEpochSecs converts epoch seconds to a time. The boolean is false when the value is out of range.
func FromEpochSecs(secs int64) (time.Time, bool) {
	if secs < minEpochSecs || secs > maxEpochSecs {
		return time.Time{}, false
	}
	return time.Unix(secs, 0).UTC(), true
}

// UsageLimitState is the resume schedule of a task stopped by a provider usage limit.
type UsageLimitState struct {
	// ResumeAt is when to retry the resume; nil when the task is not stopped by a limit.
	ResumeAt *time.Time
	// Attempts counts how many times in a row the limit was hit without completing a turn (feeds the backoff).
	Attempts uint32
}

// AfterHit returns the state after a new hit of the limit: it uses resetsAt when the provider gave one
// and it is in the future; otherwise it probes with exponential backoff (5 min up to 1 h).
func (s UsageLimitState) AfterHit(resetsAt *time.Time, now time.Time) UsageLimitState {
	var resumeAt time.Time
	if resetsAt != nil && resetsAt.After(now) {
		resumeAt = resetsAt.Add(resetMargin)
	} else {
		exponent := s.Attempts
		if exponent > maxBackoffExponent {
			exponent = maxBackoffExponent
		}
		delay := probeBaseDelay << exponent
		if delay > probeMaxDelay {
			delay = probeMaxDelay
		}
		resumeAt = now.Add(delay)
	}
	attempts := s.Attempts
	if attempts < ^uint32(0) {
		attempts++
	}
	return UsageLimitState{ResumeAt: &resumeAt, Attempts: attempts}
}

// IsDue reports whether the time to retry has come.
func (s UsageLimitState) IsDue(now time.Time) bool {
	return s.ResumeAt != nil && !s.ResumeAt.After(now)
}
