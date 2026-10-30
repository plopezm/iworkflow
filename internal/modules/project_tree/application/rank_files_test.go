package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileNamePrefixBeatsOtherMatches(t *testing.T) {
	ranked := rankFiles([]string{"docs/main_notes.md", "src/main.rs", "src/lib/model.rs"}, "main", 10)

	require.Equal(t, []string{"src/main.rs", "docs/main_notes.md"}, ranked)
}

func TestANameMatchBeatsAnEarlierDirectoryMatch(t *testing.T) {
	ranked := rankFiles([]string{"main_docs/readme.md", "src/main.rs"}, "main", 10)

	require.Equal(t, []string{"src/main.rs", "main_docs/readme.md"}, ranked)
}

func TestEmptyQueryListsShortestPathsUpToTheLimit(t *testing.T) {
	ranked := rankFiles([]string{"a/b/c.rs", "z.rs", "m/n.rs"}, "", 2)

	require.Equal(t, []string{"z.rs", "m/n.rs"}, ranked)
}

func TestDirectoryFragmentsMatchCaseInsensitively(t *testing.T) {
	ranked := rankFiles([]string{"Src/Lib.rs", "other.txt"}, "src/l", 10)

	require.Equal(t, []string{"Src/Lib.rs"}, ranked)
}

func TestNoMatchReturnsAnEmptyList(t *testing.T) {
	ranked := rankFiles([]string{"a.rs"}, "zzz", 10)

	require.Empty(t, ranked)
}
