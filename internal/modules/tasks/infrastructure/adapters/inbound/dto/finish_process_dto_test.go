package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestFinishProcessFromDomainKeepsTheRowAndEncodesItsIdentity(t *testing.T) {
	t.Parallel()
	process := models.WorktreeProcess{
		PID:     42,
		Name:    "node",
		Path:    "/wt/app/server.js",
		Reason:  models.ProcessReasonExecutable,
		Started: "Thu Nov 19 10:00:00 2026",
	}

	got := FinishProcessFromDomain(process)

	assert.Equal(t, 42, got.PID)
	assert.Equal(t, "node", got.Name)
	assert.Equal(t, "/wt/app/server.js", got.Path)
	assert.Equal(t, models.ProcessReasonExecutable, got.Reason)
	assert.Equal(t, "42|Thu Nov 19 10:00:00 2026", got.Value, "the checkbox posts the pid and the start token")
}

func TestDecodeFinishSelectionWithoutValuesSelectsNothing(t *testing.T) {
	t.Parallel()

	selected, err := DecodeFinishSelection(nil)

	require.NoError(t, err)
	assert.Empty(t, selected, "an empty selection closes no process")
}

func TestDecodeFinishSelectionReadsEachPidAndStartToken(t *testing.T) {
	t.Parallel()
	values := []string{"42|Thu Nov 19 10:00:00 2026", "77|token|with|bars"}

	selected, err := DecodeFinishSelection(values)

	require.NoError(t, err)
	assert.Equal(t, []models.WorktreeProcess{
		{PID: 42, Started: "Thu Nov 19 10:00:00 2026"},
		{PID: 77, Started: "token|with|bars"},
	}, selected, "only the identity travels; the closer re-checks the process")
}

func TestDecodeFinishSelectionAcceptsAnEmptyStartToken(t *testing.T) {
	t.Parallel()
	// Some platforms cannot read the start time of a process; the finish must not be blocked by it.
	selected, err := DecodeFinishSelection([]string{"42|"})

	require.NoError(t, err)
	assert.Equal(t, []models.WorktreeProcess{{PID: 42}}, selected)
}

func TestDecodeFinishSelectionRefusesEveryMalformedValue(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"no separator":            {"42"},
		"empty value":             {""},
		"pid is not a number":     {"abc|token"},
		"pid is zero":             {"0|token"},
		"pid is negative":         {"-3|token"},
		"pid is empty":            {"|token"},
		"one good, one malformed": {"42|token", "x|token"},
	}
	for name, values := range cases {
		selected, err := DecodeFinishSelection(values)

		require.Error(t, err, name)
		assert.Nil(t, selected, name)
	}
}

func TestDecodeFinishSelectionRefusesTheSamePidTwice(t *testing.T) {
	t.Parallel()

	selected, err := DecodeFinishSelection([]string{"42|token-a", "42|token-b"})

	require.Error(t, err)
	assert.Nil(t, selected, "a duplicated pid is refused rather than closed twice")
}
