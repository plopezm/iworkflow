// Package system adapts the process and the operating system (network interfaces, restart handle,
// listen address) to the preferences ports.
package system

import (
	"context"
	"log/slog"

	"iworkflow/internal/core/listen"
	"iworkflow/internal/modules/preferences/domain/models"
)

// NetworkDetector reports this machine's addresses through internal/core/listen.
type NetworkDetector struct {
	logger *slog.Logger
}

// NewNetworkDetector builds the detector; logger receives lookup failures.
func NewNetworkDetector(logger *slog.Logger) *NetworkDetector {
	return &NetworkDetector{logger: logger}
}

// Interfaces lists the IPv4 interfaces in the order the OS reports them. A failed lookup is logged
// and reported as empty.
func (d *NetworkDetector) Interfaces(ctx context.Context) []models.NetworkInterface {
	found, err := listen.LocalNetworkInterfaces(ctx)
	if err != nil {
		d.logger.Warn("could not list network interfaces", "error", err)
		return nil
	}
	interfaces := make([]models.NetworkInterface, 0, len(found))
	for _, item := range found {
		interfaces = append(interfaces, models.NetworkInterface{Name: item.Name, Address: item.IP.String()})
	}
	return interfaces
}

// PreferredAddress is the address the OS routes through, if it has one.
func (d *NetworkDetector) PreferredAddress(ctx context.Context) (string, bool) {
	ip, ok := listen.LocalNetworkIP(ctx)
	if !ok {
		return "", false
	}
	return ip.String(), true
}
