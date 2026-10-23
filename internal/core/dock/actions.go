package dock

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// ErrUnknownAction is returned for an operation the dock does not define.
var ErrUnknownAction = errors.New("unknown dock action")

// Request carries the values of one dock operation. Zero values mean "absent".
type Request struct {
	Path   string
	Title  string
	Reuse  string
	Group  GroupID
	Tab    TabID
	Target TabID
	Split  SplitID
	After  bool
	Zone   DropZone
	Dir    SplitDir
	Ratio  float64
}

var zoneNames = map[string]DropZone{"center": Center, "left": Left, "right": Right, "top": Top, "bottom": Bottom}

// ParseRequest reads the form values sent by web/static/js/dock.js. A malformed number is an error.
func ParseRequest(values url.Values) (Request, error) {
	var req Request
	req.Path = values.Get("path")
	req.Title = values.Get("title")
	req.Reuse = values.Get("reuse")
	req.After = values.Get("after") == "1"
	req.Dir = Row
	if values.Get("dir") == "column" || values.Get("dir") == string(Column) {
		req.Dir = Column
	}
	req.Zone = zoneNames[values.Get("zone")]
	var err error
	if req.Group, err = parseID[GroupID](values, "group"); err != nil {
		return Request{}, err
	}
	if req.Tab, err = parseID[TabID](values, "tab"); err != nil {
		return Request{}, err
	}
	if req.Target, err = parseID[TabID](values, "target"); err != nil {
		return Request{}, err
	}
	if req.Split, err = parseID[SplitID](values, "split"); err != nil {
		return Request{}, err
	}
	if raw := values.Get("ratio"); raw != "" {
		ratio, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil {
			return Request{}, fmt.Errorf("parse ratio: %w", parseErr)
		}
		req.Ratio = ratio
	}
	return req, nil
}

type identifier interface {
	~uint64
}

func parseID[T identifier](values url.Values, key string) (T, error) {
	raw := values.Get(key)
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return T(parsed), nil
}

// Apply performs operation op on layout. It mirrors the navigation actions of the Rust dock:
// opening (reusing a tab when asked), moving, closing, splitting and renaming. A path without a
// title takes the route default. Unknown operations return ErrUnknownAction.
func Apply(ctx context.Context, layout *Layout, registry *Registry, op string, req Request) error {
	switch op {
	case "open":
		if req.Path == "" {
			return missing(op, "path")
		}
		layout.Open(req.Path, titleOf(ctx, registry, req), req.Group)
	case "open_beside":
		if req.Path == "" {
			return missing(op, "path")
		}
		if req.Group == 0 {
			layout.Open(req.Path, titleOf(ctx, registry, req), 0)
			return nil
		}
		layout.OpenBeside(req.Path, titleOf(ctx, registry, req), req.Group)
	case "open_or_reuse":
		if req.Path == "" {
			return missing(op, "path")
		}
		if _, tab, ok := layout.FindPathWhere(func(p string) bool { return registry.MatchesName(p, req.Reuse) }); ok {
			layout.Activate(tab)
			return nil
		}
		layout.Open(req.Path, titleOf(ctx, registry, req), req.Group)
	case "activate":
		layout.Activate(req.Tab)
	case "focus":
		if layout.Focused != req.Group {
			layout.Focus(req.Group)
		}
	case "close_tab":
		layout.CloseTab(req.Tab)
	case "close_others":
		layout.CloseOthers(req.Tab)
	case "close_group":
		layout.CloseGroup(req.Group)
	case "split_group":
		layout.SplitGroup(req.Group, req.Dir)
	case "move_tab":
		layout.MoveTab(req.Tab, req.Group, req.Zone)
	case "reorder_tab":
		layout.ReorderTab(req.Tab, req.Target, req.After)
	case "move_to_new_split":
		layout.MoveToNewSplit(req.Tab, req.Dir)
	case "set_ratio":
		layout.SetRatio(req.Split, req.Ratio)
	case "set_title":
		if req.Path == "" {
			return missing(op, "path")
		}
		layout.SetTitle(req.Path, req.Title)
	case "replace":
		if req.Path == "" {
			return missing(op, "path")
		}
		if req.Tab == 0 {
			layout.Open(req.Path, titleOf(ctx, registry, req), req.Group)
			return nil
		}
		layout.ReplacePath(req.Tab, req.Path, titleOf(ctx, registry, req))
	case "close_and_open":
		if req.Path == "" {
			return missing(op, "path")
		}
		layout.Open(req.Path, titleOf(ctx, registry, req), req.Group)
		layout.CloseTab(req.Tab)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownAction, op)
	}
	return nil
}

func titleOf(ctx context.Context, registry *Registry, req Request) string {
	if req.Title != "" {
		return req.Title
	}
	return registry.Title(ctx, req.Path)
}

func missing(op, field string) error {
	return fmt.Errorf("dock action %q needs %s", op, field)
}
