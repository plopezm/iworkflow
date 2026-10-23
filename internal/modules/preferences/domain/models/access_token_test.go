package models_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
)

var wellFormedToken = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestGeneratedTokensAreWellFormedAndUnique(t *testing.T) {
	first, err := models.GenerateAccessToken()
	require.NoError(t, err)
	second, err := models.GenerateAccessToken()
	require.NoError(t, err)

	require.Regexp(t, wellFormedToken, first.Value())
	_, err = models.ParseAccessToken(first.Value())
	require.NoError(t, err)
	require.NotEqual(t, first.Value(), second.Value())
}

func TestParseAccessTokenRejectsMalformedValues(t *testing.T) {
	cases := map[string]string{
		"too short":     "abc",
		"non hex":       strings.Repeat("z", 64),
		"one too long":  strings.Repeat("a", 65),
		"empty":         "",
		"space padded":  " " + strings.Repeat("a", 63),
		"one too short": strings.Repeat("a", 63),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := models.ParseAccessToken(raw)
			require.ErrorIs(t, err, domainerrors.ErrInvalidAccessToken)
		})
	}
}

func TestParseAccessTokenLowercasesHex(t *testing.T) {
	upper := strings.Repeat("AB", 32)

	token, err := models.ParseAccessToken(upper)

	require.NoError(t, err)
	require.Equal(t, strings.Repeat("ab", 32), token.Value())
}

func TestMatchesOnlyTheExactToken(t *testing.T) {
	token, err := models.GenerateAccessToken()
	require.NoError(t, err)
	other, err := models.GenerateAccessToken()
	require.NoError(t, err)

	require.True(t, token.Matches(token.Value()))
	require.False(t, token.Matches(""))
	require.False(t, token.Matches(token.Value()[1:]))
	require.False(t, token.Matches(other.Value()))
}

func TestDebugOutputHidesTheSecret(t *testing.T) {
	token, err := models.GenerateAccessToken()
	require.NoError(t, err)

	for _, rendered := range []string{
		fmt.Sprintf("%v", token),
		fmt.Sprintf("%+v", token),
		fmt.Sprintf("%#v", token),
	} {
		require.NotContains(t, rendered, token.Value())
		require.Contains(t, rendered, "AccessToken(…)")
	}
}
