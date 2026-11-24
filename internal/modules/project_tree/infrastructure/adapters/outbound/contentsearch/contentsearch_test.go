package contentsearch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
	return root
}

func query(t *testing.T, text string, regex bool, cs bool, word bool) models.ContentQuery {
	t.Helper()
	q, err := models.NewContentQuery(text, regex, cs, word)
	require.NoError(t, err)
	return q
}

// searchers returns every implementation; the contract below must hold for all of them.
func searchers(t *testing.T) map[string]outbound.ContentSearchPort {
	t.Helper()
	found := map[string]outbound.ContentSearchPort{"walk": NewWalkSearcher()}
	if binary, err := exec.LookPath("rg"); err == nil {
		found["rg"] = NewRipgrepSearcher(binary)
	} else {
		t.Log("rg not installed: only the walker is checked")
	}
	return found
}

func TestSearchersShareTheContract(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.txt":          "hello World\nnothing\nWORLD again world\n",
		"src/b.go":       "package b // world\n",
		"src/c.go":       "áé world\n",
		"bin.dat":        "world\x00binary",
		".github/ci.yml": "world: true\n",
		"empty.txt":      "",
	})
	for name, searcher := range searchers(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			got, err := searcher.Search(ctx, root, query(t, "world", false, false, false))
			require.NoError(t, err)
			paths := make([]string, 0, len(got.Files))
			for _, f := range got.Files {
				paths = append(paths, f.Path)
			}
			require.Equal(t, []string{".github/ci.yml", "a.txt", "src/b.go", "src/c.go"}, paths, "sorted, hidden included, binary excluded")
			a := got.Files[1]
			require.Equal(t, 3, a.Total)
			require.Equal(t, models.ContentMatch{Line: 1, Col: 7, Len: 5, Snippet: "hello World"}, a.Matches[0])
			require.Equal(t, 3, a.Matches[1].Line)
			require.Equal(t, 1, a.Matches[1].Col)
			require.Equal(t, 13, a.Matches[2].Col)
			require.Equal(t, 4, got.Files[3].Matches[0].Col, "columns count characters, not bytes")

			cs, err := searcher.Search(ctx, root, query(t, "world", false, true, false))
			require.NoError(t, err)
			require.Len(t, cs.Files, 4)
			require.Equal(t, 1, cs.Files[1].Total)

			none, err := searcher.Search(ctx, root, query(t, "zzz-nothing", false, false, false))
			require.NoError(t, err)
			require.Empty(t, none.Files)

			re, err := searcher.Search(ctx, root, query(t, `wor.d$`, true, false, false))
			require.NoError(t, err)
			require.NotEmpty(t, re.Files)
		})
	}
}

func TestSearchersCapMatchesAndFiles(t *testing.T) {
	files := map[string]string{"many.txt": strings.Repeat("x\n", models.MaxMatchesPerFile+20)}
	for i := 0; i < models.MaxContentFiles+5; i++ {
		files[fmt.Sprintf("f/%04d.txt", i)] = "x\n"
	}
	root := writeTree(t, files)
	for name, searcher := range searchers(t) {
		t.Run(name, func(t *testing.T) {
			got, err := searcher.Search(context.Background(), root, query(t, "x", false, false, false))
			require.NoError(t, err)
			require.Len(t, got.Files, models.MaxContentFiles)
			require.True(t, got.Truncated)
			require.Equal(t, "f/0000.txt", got.Files[0].Path)
			many := writeTree(t, map[string]string{"many.txt": files["many.txt"]})
			one, err := searcher.Search(context.Background(), many, query(t, "x", false, false, false))
			require.NoError(t, err)
			require.Equal(t, models.MaxMatchesPerFile+20, one.Files[0].Total)
			require.Len(t, one.Files[0].Matches, models.MaxMatchesPerFile)
		})
	}
}

func TestSearchersStopWhenCancelled(t *testing.T) {
	root := writeTree(t, map[string]string{"a.txt": "x\n"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, searcher := range searchers(t) {
		t.Run(name, func(t *testing.T) {
			_, err := searcher.Search(ctx, root, query(t, "x", false, false, false))
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestChoosePrefersRipgrepAndFallsBack(t *testing.T) {
	require.IsType(t, &RipgrepSearcher{}, choose(func(string) (string, error) { return "/usr/bin/rg", nil }))
	require.IsType(t, WalkSearcher{}, choose(func(string) (string, error) { return "", exec.ErrNotFound }))
}

func TestSnippetCutsAtARuneBoundary(t *testing.T) {
	long := strings.Repeat("é", maxSnippetBytes)
	got := snippetOf(long + "\n")
	require.LessOrEqual(t, len(got), maxSnippetBytes)
	require.True(t, strings.HasSuffix(got, "é"))
}
