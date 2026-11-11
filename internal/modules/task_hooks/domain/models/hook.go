// Package models holds the entities of the task_hooks module.
package models

import (
	"fmt"
	"strings"
	"time"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
)

// Phase says whether a hook runs before or after its event.
type Phase string

// Event is the task lifecycle point a hook is attached to.
type Event string

// Kind is how a hook does its work.
type Kind string

// FailurePolicy says what a failed hook does to the action it surrounds.
type FailurePolicy string

// Values of the closed sets above.
const (
	PhasePre  Phase = "pre"
	PhasePost Phase = "post"

	EventTaskStart  Event = "task_start"
	EventTaskFinish Event = "task_finish"

	KindScript Kind = "script"
	KindAgent  Kind = "agent"

	// FailureBlock aborts the action, FailureWarn reports and continues, FailureIgnore continues silently.
	FailureBlock  FailurePolicy = "block"
	FailureWarn   FailurePolicy = "warn"
	FailureIgnore FailurePolicy = "ignore"

	// MaxTimeout bounds a hook so a stuck one cannot hold a task forever.
	MaxTimeout time.Duration = 24 * time.Hour
)

// EnvVar is one environment variable added to the process of a script hook.
type EnvVar struct {
	Name  string
	Value string
}

// ScriptSpec is the work of a script hook. Args may hold templates such as {{.Branch}}; the command runs
// without a shell unless UseShell is set.
type ScriptSpec struct {
	Command  string
	Args     []string
	Env      []EnvVar
	UseShell bool
	// WorkingDir is where the command runs. Empty means the directory of the workspace (the worktree of the task);
	// a relative path is resolved against it.
	WorkingDir string
}

// AgentSpec is the work of an agent hook: a natural-language prompt and an optional choice of who runs it.
// Empty override fields mean the workspace default agent.
type AgentSpec struct {
	Prompt   string
	Runtime  string
	Model    string
	Thinking string
	Profile  string
}

// HookParams are the raw fields NewHook validates.
type HookParams struct {
	ID      string
	Name    string
	Kind    Kind
	Timeout time.Duration
	Script  ScriptSpec
	Agent   AgentSpec
}

// Hook is a reusable action of the library: what it does and for how long. Where and when it runs is decided by
// its assignments.
type Hook struct {
	id      string
	name    string
	kind    Kind
	timeout time.Duration
	script  ScriptSpec
	agent   AgentSpec
}

// NewHook validates the parameters and builds the hook. Only the spec of its kind is kept: a script needs a
// command, an agent needs a prompt; the timeout is required and at most MaxTimeout.
func NewHook(p HookParams) (Hook, error) {
	name := strings.TrimSpace(p.Name)
	switch {
	case strings.TrimSpace(p.ID) == "":
		return Hook{}, invalid("the id is required")
	case name == "":
		return Hook{}, invalid("the name is required")
	case p.Timeout <= 0:
		return Hook{}, invalid("the timeout is required")
	case p.Timeout > MaxTimeout:
		return Hook{}, invalid("the timeout is too long")
	}
	hook := Hook{id: strings.TrimSpace(p.ID), name: name, kind: p.Kind, timeout: p.Timeout}
	switch p.Kind {
	case KindScript:
		script, err := newScript(p.Script)
		if err != nil {
			return Hook{}, err
		}
		hook.script = script
	case KindAgent:
		prompt := strings.TrimSpace(p.Agent.Prompt)
		if prompt == "" {
			return Hook{}, invalid("the prompt is required")
		}
		hook.agent = AgentSpec{
			Prompt:   prompt,
			Runtime:  strings.TrimSpace(p.Agent.Runtime),
			Model:    strings.TrimSpace(p.Agent.Model),
			Thinking: strings.TrimSpace(p.Agent.Thinking),
			Profile:  strings.TrimSpace(p.Agent.Profile),
		}
	default:
		return Hook{}, invalid(fmt.Sprintf("%q is not a valid kind", p.Kind))
	}
	return hook, nil
}

func newScript(spec ScriptSpec) (ScriptSpec, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return ScriptSpec{}, invalid("the command is required")
	}
	if strings.ContainsRune(command, 0) {
		return ScriptSpec{}, invalid("the command is not valid")
	}
	workingDir := strings.TrimSpace(spec.WorkingDir)
	if strings.ContainsRune(workingDir, 0) {
		return ScriptSpec{}, invalid("the working directory is not valid")
	}
	args := make([]string, 0, len(spec.Args))
	for _, arg := range spec.Args {
		if strings.TrimSpace(arg) != "" {
			args = append(args, arg)
		}
	}
	env := make([]EnvVar, 0, len(spec.Env))
	for _, variable := range spec.Env {
		variable.Name = strings.TrimSpace(variable.Name)
		if variable.Name == "" || strings.ContainsAny(variable.Name, "= \t") {
			return ScriptSpec{}, invalid(fmt.Sprintf("%q is not a valid variable name", variable.Name))
		}
		env = append(env, variable)
	}
	return ScriptSpec{Command: command, Args: args, Env: env, UseShell: spec.UseShell, WorkingDir: workingDir}, nil
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", hookerrors.ErrInvalidHook, message)
}

// ID is the stable identifier of the hook.
func (h Hook) ID() string { return h.id }

// Name is the label of the hook in the library.
func (h Hook) Name() string { return h.name }

// Kind is script or agent.
func (h Hook) Kind() Kind { return h.kind }

// Timeout is the longest the hook may run; exceeding it counts as a failure.
func (h Hook) Timeout() time.Duration { return h.timeout }

// Script returns a copy of the script spec (empty for an agent hook).
func (h Hook) Script() ScriptSpec {
	s := h.script
	s.Args = append([]string(nil), s.Args...)
	s.Env = append([]EnvVar(nil), s.Env...)
	return s
}

// Agent returns the agent spec (empty for a script hook).
func (h Hook) Agent() AgentSpec { return h.agent }

// Params returns the fields the hook was built from, so a change can be validated again through NewHook.
func (h Hook) Params() HookParams {
	return HookParams{ID: h.id, Name: h.name, Kind: h.kind, Timeout: h.timeout, Script: h.Script(), Agent: h.agent}
}
