package pages_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/pages"
)

func TestConversionBranchSelectIsMarkedSearchable(t *testing.T) {
	var out bytes.Buffer
	branches := []dto.ConversionBranchDTO{{Name: "main"}, {Name: "feature/login", IsRemote: true}}
	require.NoError(t, pages.ConversionBranchContainer(branches).Render(context.Background(), &out))
	body := out.String()

	field := strings.Index(body, `name="base_branch"`)
	require.GreaterOrEqual(t, field, 0, "the base branch select is rendered")
	start := strings.LastIndex(body[:field], "<select")
	end := strings.Index(body[field:], ">")
	require.GreaterOrEqual(t, start, 0)
	require.GreaterOrEqual(t, end, 0)
	tag := body[start : field+end+1]

	assert.Contains(t, tag, " data-searchable", "the select is upgraded into a combobox by web/static/js/searchable_select.js")
	assert.Contains(t, tag, `data-search-placeholder=`)
	assert.Contains(t, tag, `data-search-empty=`)
	assert.Contains(t, tag, `name="base_branch"`, "the select stays the submitted field")
	assert.Contains(t, body, `value="main"`, "the branches are still offered as options of the select")
}
