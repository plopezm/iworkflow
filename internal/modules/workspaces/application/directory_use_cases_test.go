package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
)

const testHome = "/home/pi"

func TestBrowseDirectoriesDefaultsToHomeWhenPathIsEmpty(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewBrowseDirectories(browser, testHome)

	listing, err := useCase.Execute(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, []string{testHome}, browser.listed)
	assert.Equal(t, testHome, listing.Path)
}

func TestBrowseDirectoriesRejectsRelativePath(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewBrowseDirectories(browser, testHome)

	_, err := useCase.Execute(context.Background(), "relative/dir")

	require.ErrorIs(t, err, werrors.PathNotAbsolute(""))
	assert.EqualError(t, err, "la ruta debe ser absoluta: relative/dir")
	assert.Empty(t, browser.listed, "a relative path must not reach the browser")
}

func TestBrowseDirectoriesDelegatesAbsolutePath(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewBrowseDirectories(browser, testHome)

	listing, err := useCase.Execute(context.Background(), "/repos/demo")

	require.NoError(t, err)
	assert.Equal(t, []string{"/repos/demo"}, browser.listed)
	assert.Equal(t, "/repos/demo", listing.Path)
}

func TestBrowseDirectoriesPropagatesBrowserErrors(t *testing.T) {
	browser := &fakeDirectoryBrowser{listErr: werrors.IO("boom")}
	useCase := application.NewBrowseDirectories(browser, testHome)

	_, err := useCase.Execute(context.Background(), "/repos/demo")

	assert.EqualError(t, err, "error de E/S: boom")
}

func TestCreateDirectoryPassesTrimmedName(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewCreateDirectory(browser)

	created, err := useCase.Execute(context.Background(), "/repos", "  demo ")

	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"/repos", "demo"}}, browser.created)
	assert.Equal(t, "/repos/demo", created)
}

func TestCreateDirectoryRejectsInvalidNames(t *testing.T) {
	names := []string{
		"", "  ", ".", "..", "a/b", "a\\b", "a\x00b", "a\nb",
		// 256 bytes exceeds the 255-byte limit.
		strings.Repeat("a", 256),
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			browser := &fakeDirectoryBrowser{}
			useCase := application.NewCreateDirectory(browser)

			_, err := useCase.Execute(context.Background(), "/repos", name)

			require.ErrorIs(t, err, werrors.InvalidDirectoryName(""))
			assert.Empty(t, browser.created)
		})
	}
}

func TestCreateDirectoryAcceptsNameAtByteLimit(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewCreateDirectory(browser)

	_, err := useCase.Execute(context.Background(), "/repos", strings.Repeat("a", 255))

	require.NoError(t, err)
}

func TestCreateDirectoryRejectsRelativeParent(t *testing.T) {
	browser := &fakeDirectoryBrowser{}
	useCase := application.NewCreateDirectory(browser)

	_, err := useCase.Execute(context.Background(), "rel", "demo")

	require.ErrorIs(t, err, werrors.PathNotAbsolute(""))
	assert.Empty(t, browser.created)
}

func TestCreateDirectoryPropagatesBrowserErrors(t *testing.T) {
	browser := &fakeDirectoryBrowser{createErr: werrors.IO("boom")}
	useCase := application.NewCreateDirectory(browser)

	_, err := useCase.Execute(context.Background(), "/repos", "demo")

	assert.EqualError(t, err, "error de E/S: boom")
}
