package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThemeStorageKeyIsTheOriginalKey(t *testing.T) {
	assert.Equal(t, "pi-theme", ThemeStorageKey)
}
