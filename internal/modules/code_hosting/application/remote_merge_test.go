package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/application"
	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/ports/inbound"
)

func boundRemoteMerge(f *fixture) *application.RemoteMerge {
	remote := application.NewRemoteMerge()
	remote.Bind(application.NewMergeTaskMergeRequest(f.resolver(), f.remote))
	return remote
}

func TestUnboundRemoteMergeReportsNoOpenRequest(t *testing.T) {
	f := newFixture(t)

	outcome, err := application.NewRemoteMerge().MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
	require.Empty(t, f.client.merged)
}

func TestBoundRemoteMergeReportsMergedWhenTheRequestIsMerged(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeMerged, outcome)
	require.Equal(t, []uint64{7}, f.client.merged)
}

func TestBoundRemoteMergeReportsNoOpenRequestWithoutAnOpenRequest(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}

func TestBoundRemoteMergeReportsAlreadyMergedWithoutPushingOrMerging(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(7, "merged")
	f.client.findResult = &merged

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeAlreadyMerged, outcome)
	require.Empty(t, f.client.merged)
	require.Empty(t, f.remote.publishes, "an already merged request is never pushed")
}

func TestBoundRemoteMergeIntegratesLocallyWhenTheBranchAdvancedAfterTheMerge(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(7, "merged")
	f.client.findResult = &merged
	f.remote.head = "newer-local-commit"

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome, "commits after the merge must not be dropped")
	require.Empty(t, f.client.merged)
	require.Empty(t, f.remote.publishes)
}

func TestBoundRemoteMergeIntegratesLocallyWhenTheProviderSendsNoHeadCommit(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(7, "merged")
	merged.HeadSHA = ""
	f.client.findResult = &merged

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}

func TestBoundRemoteMergeIntegratesLocallyWhenTheLocalBranchCannotBeRead(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(7, "merged")
	f.client.findResult = &merged
	f.remote.headErr = codeerrors.Git("sin rama")

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}

func TestBoundRemoteMergeKeepsLocalIntegrationForAClosedRequest(t *testing.T) {
	f := newFixture(t)
	closed := mergeRequest(7, "closed")
	f.client.findResult = &closed

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}

func TestBoundRemoteMergeTreatsTasksWithoutRemoteProviderAsLocal(t *testing.T) {
	f := newFixture(t)
	f.remote.originErr = codeerrors.NoOrigin("/repo")

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}

func TestBoundRemoteMergePropagatesOtherFailures(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open
	f.remote.publishErr = codeerrors.BranchPublishFailed("pi/feature", "rejected")

	outcome, err := boundRemoteMerge(f).MergeOpenRequest(context.Background(), taskID)

	require.Equal(t, codeerrors.BranchPublishFailed("pi/feature", "rejected"), err)
	require.NotEqual(t, inbound.RemoteMergeMerged, outcome)
}

func TestRemoteMergeKeepsTheFirstBinding(t *testing.T) {
	first := newFixture(t)
	second := newFixture(t)
	open := mergeRequest(7, "opened")
	second.client.findResult = &open
	remote := application.NewRemoteMerge()
	remote.Bind(application.NewMergeTaskMergeRequest(first.resolver(), first.remote))
	remote.Bind(application.NewMergeTaskMergeRequest(second.resolver(), second.remote))

	outcome, err := remote.MergeOpenRequest(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
	require.Empty(t, second.client.merged)
}
