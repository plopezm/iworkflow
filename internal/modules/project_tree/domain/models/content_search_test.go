package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestNewContentQueryRejectsEmptyText(t *testing.T) {
	_, err := models.NewContentQuery("", false, false, false)
	require.ErrorIs(t, err, treeerrors.InvalidSearch(""))
}

func TestNewContentQueryRejectsBrokenRegex(t *testing.T) {
	_, err := models.NewContentQuery("(", true, false, false)
	require.ErrorIs(t, err, treeerrors.InvalidSearch(""))
}

func TestNewContentQueryAcceptsParenthesisAsLiteral(t *testing.T) {
	query, err := models.NewContentQuery("(", false, false, false)
	require.NoError(t, err)
	require.Equal(t, "(", query.Text())
}

func TestContentQueryPatternFlags(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		flags [3]bool // regex, case, word
		in    string
		want  bool
	}{
		{"literal ignores case by default", "foo.bar", [3]bool{false, false, false}, "FOO.BAR", true},
		{"literal does not treat dot as wildcard", "foo.bar", [3]bool{false, false, false}, "fooxbar", false},
		{"regex dot is a wildcard", "foo.bar", [3]bool{true, false, false}, "fooxbar", true},
		{"case sensitive", "Foo", [3]bool{false, true, false}, "foo", false},
		{"whole word rejects a substring", "foo", [3]bool{false, false, true}, "foobar", false},
		{"whole word accepts a delimited word", "foo", [3]bool{false, false, true}, "a foo b", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := models.NewContentQuery(tt.text, tt.flags[0], tt.flags[1], tt.flags[2])
			require.NoError(t, err)
			pattern, err := query.Pattern()
			require.NoError(t, err)
			require.Equal(t, tt.want, pattern.MatchString(tt.in))
		})
	}
}
