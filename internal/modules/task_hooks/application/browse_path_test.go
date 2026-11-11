package application_test

import (
	"context"
	"path/filepath"
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

// An empty or relative path has no folder of its own to start from, so the browser gets an empty path and starts at the
// home folder; an absolute one is listed as it is.
func TestBrowsePathStartsAtTheHomeFolderUnlessGivenAnAbsolutePath(t *testing.T) {
	absolute := filepath.Join(string(filepath.Separator), "etc")
	tests := map[string]struct{ in, want string }{
		"empty":      {in: "", want: ""},
		"blank":      {in: "  ", want: ""},
		"relative":   {in: "scripts", want: ""},
		"absolute":   {in: absolute, want: absolute},
		"padded abs": {in: " " + absolute + " ", want: absolute},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			browser := &recordingBrowser{dir: "unset"}
			_, err := application.NewBrowsePath(browser).Execute(context.Background(), application.BrowsePathInput{Path: tc.in})
			require.NoError(t, err)
			assert.Equal(t, tc.want, browser.dir)
		})
	}
}
