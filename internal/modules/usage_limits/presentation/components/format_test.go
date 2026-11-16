package components

import (
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
)

func TestResetTextUsesMinutesHoursAndDays(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "minutes under an hour", in: 45 * time.Minute, want: "45m"},
		{name: "hours and padded minutes", in: 2*time.Hour + 10*time.Minute, want: "2h10"},
		{name: "last hour under two days", in: 47*time.Hour + 59*time.Minute, want: "47h59"},
		{name: "days from forty-eight hours", in: 48 * time.Hour, want: "2d"},
		{name: "days", in: 3 * 24 * time.Hour, want: "3d"},
		{name: "days round to the nearest one", in: 2*24*time.Hour + 23*time.Hour, want: "3d"},
		{name: "days round down below half a day", in: 2*24*time.Hour + 11*time.Hour, want: "2d"},
		{name: "zero", in: 0, want: "0m"},
		{name: "negative is zero", in: -time.Minute, want: "0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resetText(tt.in))
		})
	}
}

func TestPercentTextAndFillStyle(t *testing.T) {
	assert.Equal(t, "13%", percentText(13))
	assert.Equal(t, templ.SafeCSS("width: 13%"), fillStyle(13))
}

func TestWindowLabelKeepsTheFiveHourNameAndTranslatesTheWeek(t *testing.T) {
	assert.Equal(t, "5h", windowLabel(t.Context(), "5h"))
	assert.Equal(t, "7d", windowLabel(t.Context(), "week"))
}
