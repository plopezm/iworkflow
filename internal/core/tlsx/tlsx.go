// Package tlsx builds the optional TLS configuration of the server. Browsers only expose the microphone in a secure
// context, so a server reached over the network needs HTTPS for the voice dictation of the prompt.
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
	"strings"
	"time"
)

const (
	// CertVar and KeyVar name the PEM files of a certificate the operator provides.
	CertVar string = "IWF_TLS_CERT"
	KeyVar  string = "IWF_TLS_KEY"
	// AutoVar set to "auto" makes the server create and reuse a self-signed certificate in the data folder.
	AutoVar string = "IWF_TLS"

	autoValue     string        = "auto"
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

// Config returns the TLS configuration, or nil when HTTPS is not enabled. dir is where the self-signed certificate
// lives and hosts are the extra names or addresses it must be valid for.
func Config(getenv func(string) string, dir string, hosts []string, now time.Time) (*tls.Config, error) {
	cert, key := strings.TrimSpace(getenv(CertVar)), strings.TrimSpace(getenv(KeyVar))
	if (cert == "") != (key == "") {
		return nil, ErrIncomplete
	}
	if cert == "" {
		if strings.TrimSpace(getenv(AutoVar)) != autoValue {
			return nil, nil
		}
		cert, key = filepath.Join(dir, certFile), filepath.Join(dir, keyFile)
		if err := ensureSelfSigned(cert, key, hosts, now); err != nil {
			return nil, err
		}
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("load the TLS certificate: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}, nil
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
