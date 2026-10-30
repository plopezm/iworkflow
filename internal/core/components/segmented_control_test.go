package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectedSegmentIsMarked(t *testing.T) {
	require.Equal(t, "segment", segmentClass(false))
	require.Equal(t, "segment segment-selected", segmentClass(true))
}
