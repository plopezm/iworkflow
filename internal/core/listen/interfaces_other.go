//go:build !unix

package listen

import "context"

// localNetworkInterfaces offers only the address the OS routes through, as the original did on
// non-Unix systems.
func localNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	ip, ok := LocalNetworkIP(ctx)
	if !ok {
		return nil, nil
	}
	return reachableInterfaces([]candidateInterface{{Name: "default", IP: ip, Up: true}}), nil
}
