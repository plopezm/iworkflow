package components

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/a-h/templ"
)

// resetText is the compact time left until a reset: minutes under an hour ("45m"), hours and two-digit
// minutes under 48 hours ("2h10"), then days rounded to the nearest one ("3d"). A passed reset reads as "0m".
func resetText(in time.Duration) string {
	if in < 0 {
		in = 0
	}
	switch {
	case in < time.Hour:
		return fmt.Sprintf("%dm", int(in.Minutes()))
	case in < 48*time.Hour:
		hours := int(in.Hours())
		minutes := int(in.Minutes()) % 60
		return fmt.Sprintf("%dh%02d", hours, minutes)
	default:
		return fmt.Sprintf("%dd", int(math.Round(in.Hours()/24)))
	}
}

// percentText is the percentage shown beside a bar.
func percentText(pct int) string {
	return strconv.Itoa(pct) + "%"
}

// fillStyle sets the width of the filled part of a bar. pct is an integer between 0 and 100.
func fillStyle(pct int) templ.SafeCSS {
	return templ.SafeCSS("width: " + strconv.Itoa(pct) + "%")
}

// windowLabel is the name shown for a window: the five-hour identifier as it is, the week translated.
func windowLabel(ctx context.Context, name string) string {
	if name == windowWeek {
		return msgWeek.T(ctx)
	}
	return name
}

// unavailableText is the muted line of a provider that could not be read.
func unavailableText(ctx context.Context, name string) string {
	return msgUnavailable.Fmt(ctx, map[string]string{"name": name})
}
