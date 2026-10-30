package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

func strPtr(value string) *string {
	return &value
}

func issueIn(key string, state models.IssueLocalState) models.Issue {
	issue := sampleIssue(key)
	issue.LocalState = state
	return issue
}

func TestListIssuesSplitsIssuesByLaneKeepingOrderAndAllowingOverlap(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{sampleIssue("A-1"), sampleIssue("A-2"), sampleIssue("A-3")}}
	issues.issues[0].Remote.Status = "To Do"
	lanes := &fakeLanes{lanes: []models.Lane{
		{ID: "mine", Name: "Mine"},
		{ID: "l2", Name: "Second"},
		{ID: "empty", Name: "Empty"},
	}}
	listing, err := application.NewListIssues(&membershipIssues{fakeIssues: issues, memberships: map[string][]string{
		"mine": {"A-1", "A-2", "A-3"},
		"l2":   {"A-3"},
	}}, lanes).Execute(context.Background(), models.IssueFilter{})

	require.NoError(t, err)
	assert.Equal(t, 3, listing.Total)
	assert.Equal(t, []string{"To Do"}, listing.Statuses)
	assert.Equal(t, []string{"A-1", "A-2", "A-3"}, keysOf(listing.Lanes[0].Issues))
	assert.Equal(t, []string{"A-3"}, keysOf(listing.Lanes[1].Issues))
	assert.Empty(t, listing.Lanes[2].Issues)
}

func TestListIssuesAppliesTheFilterInsideEachLaneButKeepsUnfilteredTotals(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{
		sampleIssue("A-1"),
		issueIn("A-2", models.IssueLocalStateIgnored),
		sampleIssue("A-3"),
	}}
	lanes := &fakeLanes{lanes: []models.Lane{{ID: "mine"}, {ID: "l2"}}}
	state := models.IssueLocalStateNew

	listing, err := application.NewListIssues(&membershipIssues{fakeIssues: issues, memberships: map[string][]string{
		"mine": {"A-1", "A-2", "A-3"},
		"l2":   {"A-2"},
	}}, lanes).Execute(context.Background(), models.IssueFilter{LocalState: &state})

	require.NoError(t, err)
	assert.Len(t, listing.Lanes[0].Issues, 2)
	assert.Equal(t, 3, listing.Lanes[0].Total)
	assert.Equal(t, 1, listing.Lanes[1].Total)
}

func TestGetIssueReturnsTheIssueOrNotFound(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{sampleIssue("A-1")}}
	useCase := application.NewGetIssue(issues)

	issue, err := useCase.Execute(context.Background(), "A-1")
	require.NoError(t, err)
	assert.Equal(t, "A-1", issue.Key())

	_, err = useCase.Execute(context.Background(), "B-2")
	require.ErrorIs(t, err, jiraerrors.IssueNotFound(""))
	require.EqualError(t, err, "tarea de Jira no encontrada: B-2")
}

func TestIgnoresANewIssue(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{sampleIssue("A-1")}}

	require.NoError(t, application.NewSetIssueIgnored(issues).Execute(context.Background(), "A-1", true))

	require.Len(t, issues.setStates, 1)
	assert.Equal(t, models.IssueLocalStateIgnored, issues.setStates[0].state)
	assert.Nil(t, issues.setStates[0].taskID)
}

func TestRestoringMakesItNewAndClearsTheLink(t *testing.T) {
	task := "task-9"
	issues := &fakeIssues{issues: []models.Issue{issueIn("A-1", models.IssueLocalStateConverted)}}
	issues.issues[0].TaskID = &task

	require.NoError(t, application.NewSetIssueIgnored(issues).Execute(context.Background(), "A-1", false))

	require.Len(t, issues.setStates, 1)
	assert.Equal(t, models.IssueLocalStateNew, issues.setStates[0].state)
	assert.Nil(t, issues.setStates[0].taskID)
}

func TestAConvertedIssueCannotBeIgnored(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{issueIn("A-1", models.IssueLocalStateConverted)}}

	err := application.NewSetIssueIgnored(issues).Execute(context.Background(), "A-1", true)

	require.ErrorIs(t, err, jiraerrors.AlreadyConverted(""))
	require.EqualError(t, err, "la tarea A-1 ya se convirtió en una tarea de iworkflow")
	assert.Empty(t, issues.setStates)
}

func TestListIssueCommentsReadsTheCommentsOfTheIssue(t *testing.T) {
	client := newFakeClient()
	client.commentsFn = func(key string) ([]models.Comment, error) {
		return []models.Comment{{ID: "1", Author: "Ana", Body: "hola " + key}}, nil
	}

	comments, err := application.NewListIssueComments(connectedRepo(t, "https://j", "t"), client).
		Execute(context.Background(), "A-1")

	require.NoError(t, err)
	assert.Equal(t, "hola A-1", comments[0].Body)
}

func TestListIssueTransitionsFailsWithoutConnection(t *testing.T) {
	client := newFakeClient()

	_, err := application.NewListIssueTransitions(&fakeConnections{}, client).Execute(context.Background(), "A-1")

	require.ErrorIs(t, err, jiraerrors.NotConfigured())
	assert.Zero(t, client.calls["transitions"])
}

func TestAddIssueCommentRefusesABlankCommentBeforeAnyCall(t *testing.T) {
	client := newFakeClient()

	err := application.NewAddIssueComment(&fakeConnections{}, client).Execute(context.Background(), "A-1", "  \n")

	require.ErrorIs(t, err, jiraerrors.EmptyComment())
	assert.Empty(t, client.calls)
}

func TestAddIssueCommentPostsTheTrimmedComment(t *testing.T) {
	client := newFakeClient()

	err := application.NewAddIssueComment(connectedRepo(t, "https://j", "t"), client).Execute(context.Background(), "A-1", " listo ")

	require.NoError(t, err)
	assert.Equal(t, []string{"A-1:listo"}, client.postedComments)
}

func transitionIssues(state models.IssueLocalState) *fakeIssues {
	return &fakeIssues{issues: []models.Issue{issueIn("A-1", state)}}
}

func TestTransitionsInJiraThenRefreshesTheLocalCopy(t *testing.T) {
	client := newFakeClient()
	client.transitionFn = func(key string, id string) error {
		if key != "A-1" || id != "31" {
			return errors.New("unexpected transition")
		}
		return nil
	}
	client.getIssueFn = func(key string) (models.RemoteIssue, error) {
		remote := sampleRemote(key)
		remote.Status = "Done"
		return remote, nil
	}
	issues := transitionIssues(models.IssueLocalStateNew)

	issue, err := application.NewTransitionIssue(connectedRepo(t, "https://j", "t"), issues, client, fixedClock(syncAt)).
		Execute(context.Background(), "A-1", "31")

	require.NoError(t, err)
	assert.Equal(t, "Done", issue.Remote.Status)
	require.Len(t, issues.updated, 1)
	assert.Equal(t, "Done", issues.updated[0].Status)
}

func TestUnknownIssueNeverReachesJira(t *testing.T) {
	client := newFakeClient()

	_, err := application.NewTransitionIssue(connectedRepo(t, "https://j", "t"), &fakeIssues{}, client, fixedClock(syncAt)).
		Execute(context.Background(), "A-1", "31")

	require.ErrorIs(t, err, jiraerrors.IssueNotFound(""))
	assert.Zero(t, client.calls["transition"])
}

func TestAJiraFailureLeavesTheLocalCopyUntouched(t *testing.T) {
	client := newFakeClient()
	client.transitionFn = func(string, string) error { return jiraerrors.Remote("HTTP 400") }
	issues := transitionIssues(models.IssueLocalStateNew)

	_, err := application.NewTransitionIssue(connectedRepo(t, "https://j", "t"), issues, client, fixedClock(syncAt)).
		Execute(context.Background(), "A-1", "31")

	require.ErrorIs(t, err, jiraerrors.Remote(""))
	assert.Empty(t, issues.updated)
}

func TestAFailedRefreshAfterTheTransitionReturnsTheStaleLocalCopy(t *testing.T) {
	client := newFakeClient()
	client.getIssueFn = func(string) (models.RemoteIssue, error) { return models.RemoteIssue{}, jiraerrors.Remote("timeout") }
	issues := transitionIssues(models.IssueLocalStateNew)

	issue, err := application.NewTransitionIssue(connectedRepo(t, "https://j", "t"), issues, client, fixedClock(syncAt)).
		Execute(context.Background(), "A-1", "31")

	require.NoError(t, err)
	assert.Equal(t, "A-1", issue.Key())
	assert.Empty(t, issues.updated)
}

func TestPrepareConversionBuildsTheDraftAndListsWorkspaces(t *testing.T) {
	issues := &fakeIssues{issues: []models.Issue{sampleIssue("A-1")}}
	workspaces := &fakeWorkspaces{workspaces: []models.WorkspaceOption{{ID: "w1", Name: "app", BaseBranch: "main"}}}

	prepared, err := application.NewPrepareConversion(issues, workspaces).Execute(context.Background(), "A-1")

	require.NoError(t, err)
	assert.Equal(t, "A-1: Resumen de A-1", prepared.Draft.Title)
	assert.Len(t, prepared.Workspaces, 1)
}

func TestPrepareConversionReportsAnUnknownIssue(t *testing.T) {
	_, err := application.NewPrepareConversion(&fakeIssues{}, &fakeWorkspaces{}).Execute(context.Background(), "A-1")
	require.EqualError(t, err, "tarea de Jira no encontrada: A-1")
}

func TestBlankWorkspaceReturnsNothingWithoutAskingTheCatalog(t *testing.T) {
	workspaces := &fakeWorkspaces{branches: []models.BranchOption{{Name: "main"}}}

	branches, err := application.NewListConversionBranches(workspaces).Execute(context.Background(), " ")

	require.NoError(t, err)
	assert.Empty(t, branches)
	assert.Empty(t, workspaces.branchOf)
}

func TestListConversionBranchesDelegatesForARealWorkspace(t *testing.T) {
	workspaces := &fakeWorkspaces{branches: []models.BranchOption{{Name: "main"}}}

	branches, err := application.NewListConversionBranches(workspaces).Execute(context.Background(), " w1 ")

	require.NoError(t, err)
	assert.Equal(t, "main", branches[0].Name)
	assert.Equal(t, []string{"w1"}, workspaces.branchOf)
}

func conversionRequest() models.ConversionRequest {
	return models.ConversionRequest{
		Key:         "A-1",
		WorkspaceID: "w1",
		BaseBranch:  strPtr("origin/develop"),
		Title:       "Título editado",
		Prompt:      "Prompt editado",
		Priority:    models.PriorityHintUrgent,
		StartNow:    true,
	}
}

func TestConvertCreatesTheTaskWithTheEditedMappingAndLinksIt(t *testing.T) {
	issues := transitionIssues(models.IssueLocalStateNew)
	tasks := &fakeTasks{result: outbound.CreatedLinkedTask{TaskID: "task-1", StartError: strPtr("sin agente")}}

	converted, err := application.NewConvertIssue(issues, tasks).Execute(context.Background(), conversionRequest())

	require.NoError(t, err)
	assert.Equal(t, "task-1", converted.TaskID)
	require.NotNil(t, converted.StartError)
	assert.Equal(t, "sin agente", *converted.StartError)
	require.Len(t, tasks.calls, 1)
	assert.Equal(t, "w1", tasks.calls[0].WorkspaceID)
	require.NotNil(t, tasks.calls[0].BaseBranch)
	assert.Equal(t, "origin/develop", *tasks.calls[0].BaseBranch)
	assert.Equal(t, "Título editado", tasks.calls[0].Title)
	assert.Equal(t, models.PriorityHintUrgent, tasks.calls[0].Priority)
	assert.True(t, tasks.calls[0].StartNow)
	require.Len(t, issues.setStates, 1)
	assert.Equal(t, models.IssueLocalStateConverted, issues.setStates[0].state)
	require.NotNil(t, issues.setStates[0].taskID)
	assert.Equal(t, "task-1", *issues.setStates[0].taskID)
}

func TestAFailingLinkNeverFailsTheConversionOfAnExistingTask(t *testing.T) {
	issues := transitionIssues(models.IssueLocalStateNew)
	issues.setErr = jiraerrors.Repository("disco lleno")
	tasks := &fakeTasks{result: outbound.CreatedLinkedTask{TaskID: "task-1"}}

	converted, err := application.NewConvertIssue(issues, tasks).Execute(context.Background(), conversionRequest())

	require.NoError(t, err)
	assert.Equal(t, "task-1", converted.TaskID)
	require.NotNil(t, converted.StartError)
	assert.Contains(t, *converted.StartError, "no se pudo enlazar")
	assert.Contains(t, *converted.StartError, "disco lleno")
	assert.Len(t, tasks.calls, 1, "the task is created once, whatever the link does")
}

func TestAnAlreadyConvertedIssueCreatesNoSecondTask(t *testing.T) {
	tasks := &fakeTasks{}

	_, err := application.NewConvertIssue(transitionIssues(models.IssueLocalStateConverted), tasks).
		Execute(context.Background(), conversionRequest())

	require.ErrorIs(t, err, jiraerrors.AlreadyConverted(""))
	assert.Empty(t, tasks.calls)
}

func TestAnInvalidMappingIsRejectedBeforeAnyLookup(t *testing.T) {
	tasks := &fakeTasks{}
	request := conversionRequest()
	request.Prompt = " "

	_, err := application.NewConvertIssue(&fakeIssues{}, tasks).Execute(context.Background(), request)

	require.ErrorIs(t, err, jiraerrors.EmptyPrompt())
	assert.Empty(t, tasks.calls)
}

func TestATaskCreationFailureLeavesTheIssueUnlinked(t *testing.T) {
	issues := transitionIssues(models.IssueLocalStateNew)
	tasks := &fakeTasks{createErr: jiraerrors.TaskCreation("workspace no existe")}

	_, err := application.NewConvertIssue(issues, tasks).Execute(context.Background(), conversionRequest())

	require.EqualError(t, err, "no se pudo crear la tarea de iworkflow: workspace no existe")
	assert.Empty(t, issues.setStates)
}

// membershipIssues overrides the lane memberships of fakeIssues with a fixed table.
type membershipIssues struct {
	*fakeIssues
	memberships map[string][]string
}

func (m *membershipIssues) LaneMemberships(_ context.Context) (map[string][]string, error) {
	return m.memberships, nil
}

func keysOf(issues []models.Issue) []string {
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		keys = append(keys, issue.Key())
	}
	return keys
}

var _ outbound.IssueRepository = (*membershipIssues)(nil)
