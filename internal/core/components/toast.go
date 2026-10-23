package components

import "github.com/a-h/templ"

// ToastAutoDismissMillis is how long a notification stays before it removes itself.
const ToastAutoDismissMillis = 8000

// ToastStackID is the id of the container that toasts are appended to (out-of-band swap target).
const ToastStackID = "toasts"

// ToastProps configures a notification. OutOfBand appends it to the ToastStackID container when
// it arrives as part of another htmx response.
type ToastProps struct {
	ID        string
	Text      string
	OutOfBand bool
}

// Attributes returns the attributes of the notification root: its behaviour and, when out of band,
// the swap target.
func (p ToastProps) Attributes() templ.Attributes {
	attrs := templ.Attributes{
		"id":                 p.ID,
		"data-toast":         "",
		"data-toast-timeout": ToastAutoDismissMillis,
	}
	if p.OutOfBand {
		attrs["hx-swap-oob"] = "beforeend:#" + ToastStackID
	}
	return attrs
}
