package refresh

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTriggerAnnouncesADataChangeToHTMX(t *testing.T) {
	recorder := httptest.NewRecorder()

	Trigger(recorder)

	assert.Equal(t, []string{DataChangedEvent}, recorder.Header().Values("HX-Trigger"))
}
