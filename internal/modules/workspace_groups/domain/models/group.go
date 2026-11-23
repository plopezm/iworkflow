// Package models holds the entities of the workspace_groups module.
package models

import (
	"fmt"
	"strings"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
)

// maxNameLength is the longest group name, in bytes.
const maxNameLength = 80

// colors are the palette keys a group can be marked with: the same list the workspaces use.
var colors = [...]string{"red", "orange", "amber", "green", "teal", "cyan", "blue", "violet", "pink", "gray"}

// Colors returns the accepted palette keys, in display order.
func Colors() []string { return append([]string(nil), colors[:]...) }

// Group is a named, optionally colored set of workspaces that the sidebar shows as a collapsible block.
type Group struct {
	id        string
	name      string
	color     string
	collapsed bool
}

// NewGroup validates the fields and builds the group. The name is trimmed and required; the color is
// optional and must be a palette key.
func NewGroup(id string, name string, color string, collapsed bool) (Group, error) {
	name = strings.TrimSpace(name)
	color = strings.TrimSpace(color)
	switch {
	case strings.TrimSpace(id) == "":
		return Group{}, invalid("the id is required")
	case name == "":
		return Group{}, invalid("the name is required")
	case len(name) > maxNameLength:
		return Group{}, invalid("the name is too long")
	case color != "" && !isColor(color):
		return Group{}, invalid(fmt.Sprintf("%q is not a palette color", color))
	}
	return Group{id: id, name: name, color: color, collapsed: collapsed}, nil
}

// ID is the stable identifier of the group.
func (g Group) ID() string { return g.id }

// Name is the label of the group.
func (g Group) Name() string { return g.name }

// Color is the palette key of the group; empty means none.
func (g Group) Color() string { return g.color }

// Collapsed reports whether the sidebar shows the group closed.
func (g Group) Collapsed() bool { return g.collapsed }

func isColor(key string) bool {
	for _, known := range colors {
		if known == key {
			return true
		}
	}
	return false
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", gerrors.ErrInvalidGroup, message)
}
