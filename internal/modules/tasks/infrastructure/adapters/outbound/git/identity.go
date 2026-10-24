//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// identityArgs returns the `-c user.*` options for a repository without identity. The fallback is only
// used when git has no name or no e-mail configured for the repository.
func identityArgs(ctx context.Context, dir string) ([]string, error) {
	email, err := runRaw(ctx, dir, "config", "user.email")
	if err != nil {
		return nil, werrors.Git(err.Error())
	}
	name, err := runRaw(ctx, dir, "config", "user.name")
	if err != nil {
		return nil, werrors.Git(err.Error())
	}
	var args []string
	if !name.success || strings.TrimSpace(name.stdout) == "" {
		args = append(args, "-c", "user.name="+fallbackUserName)
	}
	if !email.success || strings.TrimSpace(email.stdout) == "" {
		args = append(args, "-c", "user.email="+fallbackUserEmail)
	}
	return args, nil
}

// gitWithIdentity runs a command that writes commits, with the fallback identity when needed.
func gitWithIdentity(ctx context.Context, dir string, args ...string) (result, error) {
	identity, err := identityArgs(ctx, dir)
	if err != nil {
		return result{}, err
	}
	return runRaw(ctx, dir, append(identity, args...)...)
}
