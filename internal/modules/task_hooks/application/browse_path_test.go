package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

type recordingBrowser struct{ dir string }

func (b *recordingBrowser) List(_ context.Context, dir string) (models.Listing, error) {
	b.dir = dir
	return models.Listing{Path: dir}, nil
}

type workspaceAt string

func (w workspaceAt) Path(context.Context, string) (string, error) { return string(w), nil }

func TestBrowsePathResolvesAgainstTheWorkspace(t *testing.T) {
	tests := map[string]struct{ in, want string }{
		"empty":    {in: "", want: "/work/w1"},
		"relative": {in: "scripts", want: "/work/w1/scripts"},
		"absolute": {in: "/etc", want: "/etc"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			browser := &recordingBrowser{}
			_, err := application.NewBrowsePath(workspaceAt("/work/w1"), browser).Execute(context.Background(), application.BrowsePathInput{WorkspaceID: "w1", Path: tc.in})
			require.NoError(t, err)
			assert.Equal(t, tc.want, browser.dir)
		})
	}
}
