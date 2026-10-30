package application_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
)

func TestRequestServerRestartAsksTheProcessOnce(t *testing.T) {
	restarter := &fakeRestarter{}

	application.NewRequestServerRestart(restarter).Execute()

	require.Equal(t, 1, restarter.requests)
}
