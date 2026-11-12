package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestParseJiraDurationRefusesAbsurdOrOverflowingAmounts(t *testing.T) {
	for _, input := range []string{"99999999999999999999h", "9999999999999d", "1000000h", "400d"} {
		_, err := models.ParseJiraDuration(input)
		require.ErrorIs(t, err, jiraerrors.InvalidDuration(""), input)
	}
	got, err := models.ParseJiraDuration("30d")
	require.NoError(t, err)
	assert.Equal(t, 30*8*time.Hour, got)
}
