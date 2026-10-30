package models

import (
	"math"
	"strconv"
	"strings"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

// MinListenPort is the lowest port a regular user may bind: below it the OS reserves ports for
// privileged services.
const MinListenPort uint16 = 1024

// ListenPort is the TCP port the server listens on, restricted to the unprivileged range.
type ListenPort struct {
	port uint16
}

// NewListenPort validates a port number.
func NewListenPort(port uint16) (ListenPort, error) {
	if port < MinListenPort {
		return ListenPort{}, &domainerrors.InvalidPortError{Port: port}
	}
	return ListenPort{port: port}, nil
}

// ParseListenPort parses a stored or submitted value; anything that is not a valid port is
// rejected. An explicit plus sign is accepted, as the Rust build did.
func ParseListenPort(raw string) (ListenPort, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 0 || value > math.MaxUint16 {
		return ListenPort{}, &domainerrors.InvalidPortError{Port: 0}
	}
	return NewListenPort(uint16(value))
}

// Port returns the port number.
func (p ListenPort) Port() uint16 {
	return p.port
}
