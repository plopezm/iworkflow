package components_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func TestGroupOptionsKeepsFirstAppearanceOrder(t *testing.T) {
	options := []components.SelectOption{
		{Value: "a", Label: "A", Group: "Claude"},
		{Value: "b", Label: "B", Group: "Pi"},
		{Value: "c", Label: "C", Group: "Claude"},
		{Value: "d", Label: "D"},
	}

	groups := components.GroupOptions(options)

	require.Len(t, groups, 3)
	require.Equal(t, "Claude", groups[0].Name)
	require.Equal(t, []string{"a", "c"}, []string{groups[0].Options[0].Value, groups[0].Options[1].Value})
	require.Equal(t, "Pi", groups[1].Name)
	require.Equal(t, "", groups[2].Name, "ungrouped options form their own group")
}

func TestGroupOptionsOfNothingIsEmpty(t *testing.T) {
	require.Empty(t, components.GroupOptions(nil))
}
