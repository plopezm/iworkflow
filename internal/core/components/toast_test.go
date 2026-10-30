package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToastOutOfBandTargetsTheStack(t *testing.T) {
	attrs := ToastProps{ID: "toast-1", Text: "saved", OutOfBand: true}.Attributes()
	require.Equal(t, "beforeend:#toasts", attrs["hx-swap-oob"])
	require.Equal(t, "toast-1", attrs["id"])
	require.Equal(t, 8000, attrs["data-toast-timeout"])
}

func TestToastInPlaceHasNoSwapTarget(t *testing.T) {
	attrs := ToastProps{ID: "toast-2", Text: "saved"}.Attributes()
	require.NotContains(t, attrs, "hx-swap-oob")
}
