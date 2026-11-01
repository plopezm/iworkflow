package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/application"
	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
)

type fakeBrowser struct {
	listing models.Listing
	err     error
}

func (f fakeBrowser) Browse(context.Context, string) (models.Listing, error) { return f.listing, f.err }

func names(listing models.Listing) []string {
	out := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		out = append(out, entry.Name)
	}
	return out
}

func sample() models.Listing {
	return models.Listing{Path: "/bin", Entries: []models.FileEntry{
		{Name: "tools", IsDir: true}, {Name: ".hidden"}, {Name: "Code"}, {Name: "vim"},
	}}
}

func TestBrowseFilesHidesDotEntriesAndFiltersIgnoringCase(t *testing.T) {
	tests := map[string]struct {
		filter string
		want   []string
	}{
		"no filter":      {"", []string{"tools", "Code", "vim"}},
		"substring":      {"CO", []string{"Code"}},
		"dot shows them": {".", []string{".hidden"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			listing, err := application.NewBrowseFiles(fakeBrowser{listing: sample()}).Execute(context.Background(), "/bin", tt.filter)

			require.NoError(t, err)
			require.Equal(t, tt.want, names(listing))
			require.False(t, listing.Truncated)
		})
	}
}

func TestBrowseFilesTruncatesHugeFolders(t *testing.T) {
	huge := models.Listing{}
	for i := range 600 {
		huge.Entries = append(huge.Entries, models.FileEntry{Name: fmt.Sprintf("f%03d", i)})
	}

	listing, err := application.NewBrowseFiles(fakeBrowser{listing: huge}).Execute(context.Background(), "/", "")

	require.NoError(t, err)
	require.Len(t, listing.Entries, 500)
	require.True(t, listing.Truncated)
}

func TestBrowseFilesPropagatesTheBrowserError(t *testing.T) {
	_, err := application.NewBrowseFiles(fakeBrowser{err: openerrors.ErrBrowseFailed}).Execute(context.Background(), "/x", "")

	require.ErrorIs(t, err, openerrors.ErrBrowseFailed)
}
