package dock

import "errors"

// The wire types mirror the serde layout of the Rust version: externally tagged nodes
// (`{"Group":1}` or `{"Split":{...}}`), `active: null` for an empty group, snake_case `next_id`.
type layoutWire struct {
	Root    nodeWire    `json:"root"`
	Groups  []groupWire `json:"groups"`
	Focused GroupID     `json:"focused"`
	NextID  uint64      `json:"next_id"`
}

type groupWire struct {
	ID     GroupID `json:"id"`
	Tabs   []Tab   `json:"tabs"`
	Active *TabID  `json:"active"`
}

type nodeWire struct {
	Group *GroupID   `json:"Group,omitempty"`
	Split *splitWire `json:"Split,omitempty"`
}

type splitWire struct {
	ID     SplitID  `json:"id"`
	Dir    SplitDir `json:"dir"`
	Ratio  float64  `json:"ratio"`
	First  nodeWire `json:"first"`
	Second nodeWire `json:"second"`
}

var errUnknownNode = errors.New("unknown dock node")

func toWire(l *Layout) layoutWire {
	groups := make([]groupWire, 0, len(l.Groups))
	for _, g := range l.Groups {
		tabs := make([]Tab, len(g.Tabs))
		copy(tabs, g.Tabs)
		wire := groupWire{ID: g.ID, Tabs: tabs}
		if g.Active != 0 {
			active := g.Active
			wire.Active = &active
		}
		groups = append(groups, wire)
	}
	return layoutWire{
		Root:    nodeToWire(l.Root),
		Groups:  groups,
		Focused: l.Focused,
		NextID:  l.nextID,
	}
}

func nodeToWire(node Node) nodeWire {
	switch n := node.(type) {
	case Leaf:
		group := n.Group
		return nodeWire{Group: &group}
	case *Split:
		return nodeWire{Split: &splitWire{
			ID:     n.ID,
			Dir:    n.Dir,
			Ratio:  n.Ratio,
			First:  nodeToWire(n.First),
			Second: nodeToWire(n.Second),
		}}
	default:
		return nodeWire{}
	}
}

// fromWire rebuilds the model; false when the data is not a well-formed tree.
func fromWire(wire layoutWire) (*Layout, bool) {
	root, err := nodeFromWire(wire.Root)
	if err != nil {
		return nil, false
	}
	groups := make([]*Group, 0, len(wire.Groups))
	for _, gw := range wire.Groups {
		tabs := make([]Tab, len(gw.Tabs))
		copy(tabs, gw.Tabs)
		g := &Group{ID: gw.ID, Tabs: tabs}
		if gw.Active != nil {
			g.Active = *gw.Active
		}
		groups = append(groups, g)
	}
	return &Layout{
		Root:    root,
		Groups:  groups,
		Focused: wire.Focused,
		nextID:  wire.NextID,
	}, true
}

func nodeFromWire(wire nodeWire) (Node, error) {
	switch {
	case wire.Group != nil && wire.Split == nil:
		return Leaf{Group: *wire.Group}, nil
	case wire.Split != nil && wire.Group == nil:
		first, err := nodeFromWire(wire.Split.First)
		if err != nil {
			return nil, err
		}
		second, err := nodeFromWire(wire.Split.Second)
		if err != nil {
			return nil, err
		}
		return &Split{
			ID:     wire.Split.ID,
			Dir:    wire.Split.Dir,
			Ratio:  wire.Split.Ratio,
			First:  first,
			Second: second,
		}, nil
	default:
		return nil, errUnknownNode
	}
}
