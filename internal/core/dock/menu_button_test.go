package dock

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnlyTheFirstGroupCarriesTheMobileMenuButton(t *testing.T) {
	layout := New()
	layout.Open("/jira", "Jira", layout.Groups[0].ID)
	layout.OpenBeside("/", "Board", layout.Groups[0].ID)
	require.Len(t, layout.Groups, 2)
	registry := newRegistry(t, routeFixture("task", "/tasks/{id}"))
	view, err := NewView(layout, registry, nil, nil)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, Root(view).Render(context.Background(), &out))

	require.Equal(t, 1, bytes.Count(out.Bytes(), []byte(`data-target="sidebar-panel"`)))
}
