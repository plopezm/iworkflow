package outbound

import "context"

// ModelOption is a model that the routing editors can select.
type ModelOption struct {
	ID    string
	Label string
}

// ModelOptionsPort lists the available models. The composition root implements it on top of the
// model catalog of another module; this module only defines the contract.
type ModelOptionsPort interface {
	List(ctx context.Context) ([]ModelOption, error)
}
