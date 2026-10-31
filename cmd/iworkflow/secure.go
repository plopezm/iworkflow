package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"time"

	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/tlsx"
)

// secureServer is the HTTPS listener that runs next to the HTTP one.
type secureServer struct {
	addr   netip.AddrPort
	port   uint16
	config *tls.Config
	bind   func(ctx context.Context, addr string) (net.Listener, error)
}

// secureListener prepares the HTTPS listener from the environment, or returns nil when HTTPS is off. It listens on
// the host of the HTTP address, on the port IWF_TLS_PORT or the HTTP port plus 443.
func secureListener(getenv func(string) string, databaseURL string, httpAddr netip.AddrPort, bind bindFunc) (*secureServer, error) {
	hosts := []string{}
	if ip, ok := listen.LocalNetworkIP(context.Background()); ok {
		hosts = append(hosts, ip.String())
	}
	config, err := tlsx.Config(getenv, filepath.Join(filepath.Dir(databaseURL), "tls"), hosts, time.Now())
	if err != nil {
		return nil, fmt.Errorf("configure TLS: %w", err)
	}
	if config == nil {
		return nil, nil
	}
	port, err := tlsx.ResolvePort(getenv, httpAddr.Port())
	if err != nil {
		return nil, fmt.Errorf("configure TLS: %w", err)
	}
	return &secureServer{addr: netip.AddrPortFrom(httpAddr.Addr(), port), port: port, config: config, bind: bind}, nil
}

// open binds the HTTPS address and wraps it with TLS.
func (s *secureServer) open(ctx context.Context) (net.Listener, error) {
	listener, err := s.bind(ctx, s.addr.String())
	if err != nil {
		return nil, fmt.Errorf("bind %s: %w", s.addr, err)
	}
	return tls.NewListener(listener, s.config), nil
}
