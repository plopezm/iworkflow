// Package tlsx builds the TLS configuration of the server. The server is HTTPS only: HTTP/2 needs TLS, and browsers
// only expose the microphone of the voice dictation in a secure context. Only an explicit IWF_TLS=off, meant for
// tests and CI, serves plain HTTP.
package tlsx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// CertVar and KeyVar name the PEM files of a certificate the operator provides.
	CertVar string = "IWF_TLS_CERT"
	KeyVar  string = "IWF_TLS_KEY"
	// PortVar overrides the HTTPS port, which is the listen port otherwise.
	PortVar string = "IWF_TLS_PORT"
	// AutoVar set to "off" serves plain HTTP; anything else makes the server create and reuse a self-signed
	// certificate in the data folder, unless the operator provides one.
	AutoVar string = "IWF_TLS"

	offValue      string        = "off"
	certFile      string        = "cert.pem"
	keyFile       string        = "key.pem"
	validity      time.Duration = 825 * 24 * time.Hour
	renewBefore   time.Duration = 30 * 24 * time.Hour
	dirMode       os.FileMode   = 0o700
	privateMode   os.FileMode   = 0o600
	serialBitSize uint          = 128
)

// ErrIncomplete reports that only one of the certificate and the key was given.
var ErrIncomplete error = errors.New("set both " + CertVar + " and " + KeyVar)

// Config returns the TLS configuration, or nil when IWF_TLS=off. dir is where the self-signed certificate
// lives and hosts are the extra names or addresses it must be valid for.
func Config(getenv func(string) string, dir string, hosts []string, now time.Time) (*tls.Config, error) {
	cert, key := strings.TrimSpace(getenv(CertVar)), strings.TrimSpace(getenv(KeyVar))
	if (cert == "") != (key == "") {
		return nil, ErrIncomplete
	}
	if strings.TrimSpace(getenv(AutoVar)) == offValue {
		return nil, nil
	}
	if cert == "" {
		cert, key = filepath.Join(dir, certFile), filepath.Join(dir, keyFile)
		if err := ensureSelfSigned(cert, key, hosts, now); err != nil {
			return nil, err
		}
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("load the TLS certificate: %w", err)
	}
	// h2 multiplexes the event stream and the short requests over one connection, so they no longer compete for the
	// six HTTP/1.1 connections a browser opens per origin; WebSockets keep using http/1.1.
	return &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}, nil
}

// ensureSelfSigned creates the certificate and key unless a valid pair covering hosts already exists.
func ensureSelfSigned(certPath string, keyPath string, hosts []string, now time.Time) error {
	if reusable(certPath, keyPath, hosts, now) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(certPath), dirMode); err != nil {
		return fmt.Errorf("create the TLS folder: %w", err)
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate the TLS key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialBitSize))
	if err != nil {
		return fmt.Errorf("generate the certificate serial: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "iworkflow"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else if host != "" {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("sign the TLS certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("encode the TLS key: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), privateMode); err != nil {
		return fmt.Errorf("write the TLS key: %w", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), privateMode); err != nil {
		return fmt.Errorf("write the TLS certificate: %w", err)
	}
	return nil
}

// reusable reports whether the saved certificate is still valid for a while and covers every host.
func reusable(certPath string, keyPath string, hosts []string, now time.Time) bool {
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	data, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || now.Add(renewBefore).After(cert.NotAfter) {
		return false
	}
	for _, host := range hosts {
		if host != "" && cert.VerifyHostname(host) != nil {
			return false
		}
	}
	return true
}

// ErrBadPort reports an HTTPS port override that is not a number in 1024-65535.
var ErrBadPort error = errors.New("invalid HTTPS port")

// ResolvePort returns the HTTPS port: IWF_TLS_PORT when set, else listenPort.
func ResolvePort(getenv func(string) string, listenPort uint16) (uint16, error) {
	const minPort, maxPort = 1024, 65535
	raw := strings.TrimSpace(getenv(PortVar))
	if raw == "" {
		return listenPort, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q: %w", PortVar, raw, ErrBadPort)
	}
	if value < minPort || value > maxPort {
		return 0, fmt.Errorf("port %d: %w", value, ErrBadPort)
	}
	return uint16(value), nil
}
