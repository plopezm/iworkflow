package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineNumbersCoverEveryLine(t *testing.T) {
	require.Equal(t, "1", LineNumbers(""))
	require.Equal(t, "1", LineNumbers("a"))
	require.Equal(t, "1\n2\n3", LineNumbers("a\nb\n"))
}

func TestMarkClassPerMark(t *testing.T) {
	require.Equal(t, "gm-added", MarkClass(LineAdded))
	require.Equal(t, "gm-modified", MarkClass(LineModified))
	require.Equal(t, "gm-deleted", MarkClass(LineDeleted))
	require.Equal(t, "gm-none", MarkClass(""))
}
