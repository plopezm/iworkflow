package models

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTaskMemoryCarriesNoContent pins the rule that the memory panel never sees the body of a memory: the model has
// the type, the title and the date only, so no template can render a content field it does not have.
func TestTaskMemoryCarriesNoContent(t *testing.T) {
	t.Parallel()
	fields := make([]string, 0)
	typ := reflect.TypeOf(TaskMemory{})
	for i := range typ.NumField() {
		fields = append(fields, typ.Field(i).Name)
	}

	assert.Equal(t, []string{"Type", "Title", "LastSeenAt"}, fields)
}
