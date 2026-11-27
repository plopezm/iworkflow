package jira_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAKeyQueryOf100KeysReturnsEveryIssueEvenWhenTheServerPagesSmaller backs the left-behind refresh of a sync:
// it sends 100 keys in one search, so the client has to follow the pages or the keys past the first page would
// silently keep a stale status.
func TestAKeyQueryOf100KeysReturnsEveryIssueEvenWhenTheServerPagesSmaller(t *testing.T) {
	fake := newFakeJira(t)
	quoted := make([]string, 100)
	for index := range quoted {
		quoted[index] = fmt.Sprintf(`"K-%d"`, index+1)
	}

	result, err := newClient().Search(context.Background(), connectionTo(t, fake.url(), testToken), "key in ("+strings.Join(quoted, ",")+")")

	require.NoError(t, err)
	assert.Len(t, result.Issues, 100)
	assert.False(t, result.Truncated)
	assert.Equal(t, "K-100", result.Issues[99].Key)
}
