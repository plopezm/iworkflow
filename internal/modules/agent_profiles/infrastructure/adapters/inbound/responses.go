package inbound

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// respond writes the parts one after another as one fragment response. The first part is the
// swap target of the request; the others are out-of-band swaps (lists, toasts).
func respond(w http.ResponseWriter, r *http.Request, status int, parts ...templ.Component) {
	web.Render(w, r, status, join(parts...))
}

func join(parts ...templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, part := range parts {
			if err := part.Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// toast is a notification sent out of band to the toast stack of the shell.
func toast(text string) templ.Component {
	return coreui.Toast(coreui.ToastProps{ID: "toast-" + uuid.NewString(), Text: text, OutOfBand: true})
}

// statusFor maps a business error to its HTTP status: invalid input is 400, a missing profile 404, a
// conflict (name taken, active profile) 409, and anything else 500.
func statusFor(err error) int {
	switch {
	case errors.Is(err, agenterrors.InvalidProfileName("")),
		errors.Is(err, agenterrors.InvalidRoleName("")),
		errors.Is(err, agenterrors.InvalidRouting("")),
		errors.Is(err, agenterrors.InvalidImport("")):
		return http.StatusBadRequest
	case errors.Is(err, agenterrors.ProfileNotFound("")):
		return http.StatusNotFound
	case errors.Is(err, agenterrors.ProfileAlreadyExists("")),
		errors.Is(err, agenterrors.CannotDeleteActiveProfile("")),
		errors.Is(err, agenterrors.RoleAlreadyExists("")):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
