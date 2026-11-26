package state

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The confirm button of the initialization prompt resends the attempt, so the group chosen before it must travel too.
func TestResubmitJSONCarriesTheChosenGroupOnlyWhenThereIsOne(t *testing.T) {
	cases := []struct {
		name    string
		groupID string
		want    map[string]string
		absent  bool
	}{
		{name: "chosen group is resent", groupID: "g1", want: map[string]string{"group_id": "g1"}},
		{name: "no group leaves the key out", groupID: "", absent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt := InitializationPrompt{Path: "/repos/demo", Reason: ReasonNoCommits, GroupID: tc.groupID}

			var values map[string]string
			require.NoError(t, json.Unmarshal([]byte(prompt.ResubmitJSON()), &values))

			if tc.absent {
				assert.NotContains(t, values, "group_id")
				return
			}
			assert.Equal(t, tc.want["group_id"], values["group_id"])
		})
	}
}
