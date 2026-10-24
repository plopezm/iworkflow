package profiles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestParseStoreAcceptsEmptyObjectAsEmptyStore(t *testing.T) {
	store, err := parseStore([]byte("{}"))

	require.NoError(t, err)
	require.Equal(t, models.NewProfileStore(nil, nil), store)
}

func TestParseStoreRejectsWrongKindAndNonObjectRoot(t *testing.T) {
	for _, raw := range []string{`{"kind":"other"}`, `{"kind":null}`, "[]", "null", `{"active":3}`, `{"profiles":[]}`} {
		_, err := parseStore([]byte(raw))
		require.ErrorIs(t, err, agenterrors.Store(""), raw)
	}
}

func TestParseStoreNamesTheProfileWhenItIsInvalid(t *testing.T) {
	_, err := parseStore([]byte(`{"profiles":{"Ok":{"sdd-apply":{"thinking":"ultra"}}}}`))

	require.ErrorIs(t, err, agenterrors.Store(""))
	require.ErrorContains(t, err, "perfil `Ok`: ")
	require.ErrorContains(t, err, "agente `sdd-apply`")
}

func TestParseStoreReturnsInvalidActiveNameWithoutStorePrefix(t *testing.T) {
	_, err := parseStore([]byte(`{"active":"bad name"}`))

	require.Equal(t, agenterrors.InvalidProfileName("bad name"), err)
}

func TestSerializeStoreIsDeterministicAndRoundTrips(t *testing.T) {
	raw := `{"profiles":{"B":{"z":{"model":"m"},"a":"x/y"},"A":{}},"active":"B"}`
	store, err := parseStore([]byte(raw))
	require.NoError(t, err)

	first, err := serializeStore(store)
	require.NoError(t, err)
	reparsed, err := parseStore([]byte(first))
	require.NoError(t, err)
	second, err := serializeStore(reparsed)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Less(t, strings.Index(first, `"A": {`), strings.Index(first, `"B": {`))
	require.Less(t, strings.Index(first, `"a": {`), strings.Index(first, `"z": {`))
}
