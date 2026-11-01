// Package models holds the entities of the open_with module.
package models

import (
	"fmt"
	"strings"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
)

// EnvVar is one environment variable added to the process of a launcher.
type EnvVar struct {
	Name  string
	Value string
}

// Launcher is an application the user can open on the worktree of a task: the executable, optional extra
// arguments and optional environment variables.
type Launcher struct {
	id   string
	name string
	path string
	args []string
	env  []EnvVar
}

// NewLauncher validates the fields and builds the launcher. Name and path are trimmed and required; blank
// arguments are dropped; every variable needs a name without "=" or whitespace.
func NewLauncher(id string, name string, path string, args []string, env []EnvVar) (Launcher, error) {
	name = strings.TrimSpace(name)
	path = strings.TrimSpace(path)
	switch {
	case strings.TrimSpace(id) == "":
		return Launcher{}, invalid("the id is required")
	case name == "":
		return Launcher{}, invalid("the name is required")
	case path == "":
		return Launcher{}, invalid("the path is required")
	case strings.ContainsRune(path, 0):
		return Launcher{}, invalid("the path is not valid")
	}
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.TrimSpace(arg) != "" {
			kept = append(kept, arg)
		}
	}
	vars := make([]EnvVar, 0, len(env))
	for _, variable := range env {
		variable.Name = strings.TrimSpace(variable.Name)
		if variable.Name == "" || strings.ContainsAny(variable.Name, "= \t") {
			return Launcher{}, invalid(fmt.Sprintf("%q is not a valid variable name", variable.Name))
		}
		vars = append(vars, variable)
	}
	return Launcher{id: id, name: name, path: path, args: kept, env: vars}, nil
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", openerrors.ErrInvalidLauncher, message)
}

// ID is the stable identifier of the launcher.
func (l Launcher) ID() string { return l.id }

// Name is the label the user sees.
func (l Launcher) Name() string { return l.name }

// Path is the executable to run: an absolute path or a command name found in PATH.
func (l Launcher) Path() string { return l.path }

// Args returns a copy of the extra arguments.
func (l Launcher) Args() []string { return append([]string(nil), l.args...) }

// Env returns a copy of the extra environment variables.
func (l Launcher) Env() []EnvVar { return append([]EnvVar(nil), l.env...) }

// LaunchSpec is what a process starter needs to run a launcher in a directory.
type LaunchSpec struct {
	Path string
	Args []string
	Env  []EnvVar
	Dir  string
}

// SpecIn builds the launch spec of the launcher for a directory: it is the working directory and the last
// argument, so the application opens that directory.
func (l Launcher) SpecIn(dir string) LaunchSpec {
	return LaunchSpec{Path: l.path, Args: append(l.Args(), dir), Env: l.Env(), Dir: dir}
}
