package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitHubPathsEncodeSegmentsAndKeepSlashes(t *testing.T) {
	require.Equal(t, "/repos/acme/app", repoPath("acme/app"))
	require.Equal(t, "/repos/a%20b/c%23d", repoPath("a b/c#d"))
	require.Equal(t, "pi/fe%20at", refPath("pi/fe at"))
}

func TestGitHubPullStateDistinguishesMergedFromClosed(t *testing.T) {
	require.Equal(t, "opened", mapPullState("open", false))
	require.Equal(t, "merged", mapPullState("closed", true))
	require.Equal(t, "closed", mapPullState("closed", false))
}

func TestGitHubMergeStatusIsNormalised(t *testing.T) {
	for _, clean := range []string{"clean", "behind", "unstable", "has_hooks"} {
		require.Equal(t, "mergeable", mapMergeStatus(&clean))
	}
	dirty, blocked, draft, unknown := "dirty", "blocked", "draft", "unknown"
	require.Equal(t, "conflict", mapMergeStatus(&dirty))
	require.Equal(t, "blocked_status", mapMergeStatus(&blocked))
	require.Equal(t, "not_approved", mapMergeStatus(&draft))
	require.Equal(t, "checking", mapMergeStatus(&unknown))
	require.Equal(t, "checking", mapMergeStatus(nil))
}

func TestGitHubCIStatusIsNormalised(t *testing.T) {
	for _, pending := range []string{"queued", "waiting", "pending", "requested"} {
		require.Equal(t, "pending", mapCIStatus(pending, nil))
	}
	require.Equal(t, "running", mapCIStatus("in_progress", nil))
	cases := map[string]string{
		"success":         "success",
		"failure":         "failed",
		"timed_out":       "failed",
		"startup_failure": "failed",
		"cancelled":       "canceled",
		"skipped":         "skipped",
		"neutral":         "skipped",
		"action_required": "manual",
	}
	for conclusion, expected := range cases {
		value := conclusion
		require.Equal(t, expected, mapCIStatus("completed", &value), conclusion)
	}
}

func TestGitHubMergedAtAloneMarksThePullAsMerged(t *testing.T) {
	mergedAt := "2026-01-01"
	pull := pullJSON{Number: 1, Title: "T", State: "closed", HTMLURL: "u", MergedAt: &mergedAt}

	mergeRequest := pull.toDomain()

	require.Equal(t, "merged", mergeRequest.State)
	require.Equal(t, "checking", mergeRequest.MergeStatus)
}
