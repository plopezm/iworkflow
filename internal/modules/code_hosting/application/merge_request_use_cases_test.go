package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/code_hosting/application"
	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

func createInput(title string) application.CreateTaskMergeRequestInput {
	return application.CreateTaskMergeRequestInput{TaskID: taskID, Title: title}
}

func TestCreateOpensTheMergeRequestWithTheTaskDefaults(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.client.createdMR = mergeRequest(9, "opened")

	status, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput("  "))

	require.NoError(t, err)
	require.Equal(t, uint64(9), status.MergeRequest.IID)
	require.Equal(t, []models.NewMergeRequest{{
		SourceBranch: "pi/feature",
		TargetBranch: "main",
		Title:        "Mi tarea",
		Description:  "Prompt de la tarea",
	}}, f.client.created)
	require.Equal(t, [][2]string{{"pi/feature", "main"}}, f.remote.ensureCalls)
}

func TestCreateUsesTheCustomTitleWhenGiven(t *testing.T) {
	f := newFixture(t)

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput("Título propio"))

	require.NoError(t, err)
	require.Len(t, f.client.created, 1)
	require.Equal(t, "Título propio", f.client.created[0].Title)
}

func TestCreateReusesTheOpenMergeRequestWithoutCreatingAnother(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(4, "opened")
	f.client.findResult = &open

	status, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Equal(t, uint64(4), status.MergeRequest.IID)
	require.Empty(t, f.client.created)
}

func TestCreatePushesAnUnpublishedBranchBeforeCreatingTheMergeRequest(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.client.branches = map[string]bool{"main": true}
	f.client.createdMR = mergeRequest(5, "opened")

	status, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Equal(t, uint64(5), status.MergeRequest.IID)
	require.Equal(t, []publishCall{{repo: "/repo", branch: "pi/feature", project: project, token: token}}, f.remote.publishes)
	require.Equal(t, []string{"publish"}, *f.events)
}

func TestCreatePushesAnEmptyBranchAfterAddingAnEmptyCommit(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.remote.ensureAdded = true
	f.client.createdMR = mergeRequest(6, "opened")

	status, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Equal(t, uint64(6), status.MergeRequest.IID)
	require.Len(t, f.remote.publishes, 1)
}

func TestCreateFailedPushIsReportedAndNothingIsCreated(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.client.branches = map[string]bool{"main": true}
	f.remote.publishErr = codeerrors.BranchPublishFailed("pi/feature", "denied")

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.Equal(t, codeerrors.BranchPublishFailed("pi/feature", "denied"), err)
	require.Empty(t, f.client.created)
}

func TestCreateAMergedPreviousMergeRequestDoesNotBlockANewOne(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(2, "merged")
	f.client.findResult = &merged
	f.client.createdMR = mergeRequest(3, "opened")

	status, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Equal(t, uint64(3), status.MergeRequest.IID)
	require.Len(t, f.client.created, 1)
}

func TestCreateSendsARemoteTrackingBaseWithoutTheRemotePrefix(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.tasks.source.BaseBranch = "origin/develop"
	f.client.branches = map[string]bool{"develop": true, "pi/feature": true}

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Len(t, f.client.created, 1)
	require.Equal(t, "develop", f.client.created[0].TargetBranch)
}

func TestCreateKeepsEverythingAfterTheRemoteInANestedBaseName(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.tasks.source.BaseBranch = "origin/release/1.0"
	f.client.branches = map[string]bool{"release/1.0": true, "pi/feature": true}

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.NoError(t, err)
	require.Equal(t, "release/1.0", f.client.created[0].TargetBranch)
}

func TestCreateUnpublishedBaseIsReportedAndNothingIsPushedOrCreated(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.tasks.source.BaseBranch = "origin/develop"
	f.client.branches = map[string]bool{"pi/feature": true}

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.Equal(t, codeerrors.BaseBranchNotPublished("develop"), err)
	require.Empty(t, f.remote.publishes)
	require.Empty(t, f.client.created)
}

func TestCreateRejectsTheTaskBranchEqualToItsBase(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil
	f.tasks.source.BaseBranch = "origin/pi/feature"

	_, err := application.NewCreateTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), createInput(""))

	require.Equal(t, codeerrors.SameSourceAndTarget("pi/feature"), err)
	require.Empty(t, f.client.created)
}

func TestGetTaskMergeRequestReturnsNilWithoutMergeRequest(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil

	status, err := application.NewGetTaskMergeRequest(f.resolver()).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.Nil(t, status)
	require.Zero(t, f.client.pipelines)
}

func TestGetTaskMergeRequestReturnsTheMergeRequestWithItsPipeline(t *testing.T) {
	f := newFixture(t)
	found := mergeRequest(7, "opened")
	f.client.findResult = &found
	f.client.pipeline = &models.Pipeline{ID: 3, Status: "running", WebURL: "https://git.corp/p/3"}

	status, err := application.NewGetTaskMergeRequest(f.resolver()).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, project, status.ProjectPath)
	require.Equal(t, uint64(7), status.MergeRequest.IID)
	require.NotNil(t, status.Pipeline)
	require.Equal(t, uint64(3), status.Pipeline.ID)
}

func TestMergeMergesTheOpenMergeRequest(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open

	status, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, "merged", status.MergeRequest.State)
	require.Equal(t, []uint64{7}, f.client.merged)
}

func TestMergePushesTheBranchBeforeMerging(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open

	_, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, []string{"publish", "merge"}, *f.events)
	require.Equal(t, []publishCall{{repo: "/repo", branch: "pi/feature", project: project, token: token}}, f.remote.publishes)
}

func TestMergeFailedPushPreventsTheMerge(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open
	f.remote.publishErr = codeerrors.BranchPublishFailed("pi/feature", "rejected")

	_, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).ExecuteIfOpen(context.Background(), taskID)

	require.Equal(t, codeerrors.BranchPublishFailed("pi/feature", "rejected"), err)
	require.Empty(t, f.client.merged)
}

func TestMergeWithoutOpenMergeRequestMergesNothing(t *testing.T) {
	f := newFixture(t)
	merged := mergeRequest(7, "merged")
	f.client.findResult = &merged

	_, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.Equal(t, codeerrors.MergeRequestNotFound("pi/feature"), err)
	require.Empty(t, f.client.merged)
	require.Empty(t, f.remote.publishes)
}

func TestExecuteIfOpenReturnsNilWithoutOpenRequest(t *testing.T) {
	f := newFixture(t)
	f.client.findResult = nil

	status, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).ExecuteIfOpen(context.Background(), taskID)

	require.NoError(t, err)
	require.Nil(t, status)
	require.Empty(t, f.remote.publishes)
}

func TestMergeProviderRefusalIsPropagatedWithItsMessage(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open
	f.client.mergeErr = codeerrors.API(models.GitLab, 405, "Branch cannot be merged")

	_, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.Equal(t, codeerrors.API(models.GitLab, 405, "Branch cannot be merged"), err)
}

func TestMergeAPipelineFailureAfterMergingStillReportsTheMerge(t *testing.T) {
	f := newFixture(t)
	open := mergeRequest(7, "opened")
	f.client.findResult = &open
	f.client.pipelineErr = codeerrors.Network("caído")

	status, err := application.NewMergeTaskMergeRequest(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, "merged", status.MergeRequest.State)
	require.Nil(t, status.Pipeline)
}

func TestPushTaskBranchPushesToTheProjectOfTheTask(t *testing.T) {
	f := newFixture(t)

	err := application.NewPushTaskBranch(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.NoError(t, err)
	require.Equal(t, []publishCall{{repo: "/repo", branch: "pi/feature", project: project, token: token}}, f.remote.publishes)
}

func TestPushTaskBranchPropagatesThePushFailure(t *testing.T) {
	f := newFixture(t)
	f.remote.publishErr = codeerrors.BranchPublishFailed("pi/feature", "rejected")

	err := application.NewPushTaskBranch(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.Equal(t, codeerrors.BranchPublishFailed("pi/feature", "rejected"), err)
}

func TestPushTaskBranchReportsUnknownTaskAndPushesNothing(t *testing.T) {
	f := newFixture(t)
	f.tasks.source = nil

	err := application.NewPushTaskBranch(f.resolver(), f.remote).Execute(context.Background(), taskID)

	require.Equal(t, codeerrors.TaskNotFound(taskID), err)
	require.Empty(t, f.remote.publishes)
}
