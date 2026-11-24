package desktop

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strconv"
)

// ServerCertificatePEM returns the leaf certificate a loopback https server presents, PEM encoded. The shell
// hands it to the web view so that exactly this certificate is accepted, and no other.
func (a Addr) ServerCertificatePEM() ([]byte, error) {
	if !a.IsLoopback() || a.Scheme == "http" {
		return nil, errors.New("not a local https server")
	}
	target := net.JoinHostPort(a.LocalHost(), strconv.Itoa(a.Port))
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	dialer := &tls.Dialer{NetDialer: &net.Dialer{}, Config: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // reads the certificate to pin it, sends nothing
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return nil, fmt.Errorf("reading the server certificate: %w", err)
	}
	defer func() { _ = conn.Close() }()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return nil, errors.New("not a TLS connection")
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("the server presented no certificate")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[0].Raw}), nil
}
