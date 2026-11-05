package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

func TestPreviewMergeSaysWhetherTheMergeOnlyCleansUp(t *testing.T) {
	cases := []struct {
		name    string
		status  models.TaskStatus
		remote  *fakeRemote
		want    bool
		wantErr bool
	}{
		{name: "already merged on the provider", status: models.StatusAwaitingReview, remote: &fakeRemote{outcome: outbound.RemoteAlreadyMerged}, want: true},
		{name: "open request is merged remotely", status: models.StatusAwaitingReview, remote: &fakeRemote{outcome: outbound.RemoteMerged}},
		{name: "no request merges locally", status: models.StatusAwaitingReview, remote: &fakeRemote{outcome: outbound.RemoteNoOpenRequest}},
		{name: "task that cannot merge", status: models.StatusBacklog, remote: &fakeRemote{outcome: outbound.RemoteAlreadyMerged}},
		{name: "provider failure", status: models.StatusAwaitingReview, remote: &fakeRemote{err: errors.New("boom")}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task, repo, _ := mergeFixture(tc.status)
			lifecycle, _ := newLifecycle(repo)

			got, err := application.NewPreviewMerge(lifecycle, tc.remote).Execute(context.Background(), task.ID)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
			assert.Empty(t, tc.remote.asked, "the preview never merges")
		})
	}
}

func TestPreviewMergeWithoutProviderKeepsTheMergeLabel(t *testing.T) {
	task, repo, _ := mergeFixture(models.StatusAwaitingReview)
	lifecycle, _ := newLifecycle(repo)

	got, err := application.NewPreviewMerge(lifecycle, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.False(t, got)
}
