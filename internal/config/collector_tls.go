package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func CollectorCertificatePath(paths Paths) string {
	return filepath.Join(paths.Home, "collector-tls", "cert.pem")
}

func collectorPrivateKeyPath(paths Paths) string {
	return filepath.Join(paths.Home, "collector-tls", "key.pem")
}

// EnsureCollectorTLS publishes the identity as one directory, never a partial pair.
// Existing identities are validated and retained, including when validation fails.
func EnsureCollectorTLS(paths Paths, listen string) error {
	if err := EnsurePrivateHome(paths.Home); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("collector TLS listen address: %w", err)
	}
	directory := filepath.Dir(CollectorCertificatePath(paths))
	if _, err := os.Lstat(directory); err == nil {
		_, err = loadCollectorIdentity(paths, host)
		return err
	} else if !os.IsNotExist(err) {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	certificate := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "Quantum Log local collector"},
		NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "localhost" {
		if ip := net.ParseIP(host); ip != nil {
			certificate.IPAddresses = append(certificate.IPAddresses, ip)
		} else {
			certificate.DNSNames = append(certificate.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp(paths.Home, ".collector-tls-")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(filepath.Join(staging, "cert.pem"))
		_ = os.Remove(filepath.Join(staging, "key.pem"))
		_ = os.Remove(staging)
	}()
	if err := EnsurePrivateDirectory(staging); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "cert.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	if err := os.Rename(staging, directory); err != nil {
		if _, statErr := os.Lstat(directory); statErr != nil {
			return fmt.Errorf("publish collector TLS identity: %w", err)
		}
	}
	_, err = loadCollectorIdentity(paths, host)
	return err
}

func loadCollectorIdentity(paths Paths, host string) (tls.Certificate, error) {
	if err := EnsurePrivateHome(paths.Home); err != nil {
		return tls.Certificate{}, err
	}
	identity, err := tls.LoadX509KeyPair(CollectorCertificatePath(paths), collectorPrivateKeyPath(paths))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("collector TLS identity is incomplete or invalid; existing files were retained: %w", err)
	}
	certificate, err := collectorCertificate(paths)
	if err != nil {
		return tls.Certificate{}, err
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		if err := certificate.VerifyHostname(host); err != nil {
			return tls.Certificate{}, fmt.Errorf("collector TLS identity does not cover listener host; existing identity was retained: %w", err)
		}
	}
	identity.Leaf = certificate
	return identity, nil
}

func collectorCertificate(paths Paths) (*x509.Certificate, error) {
	contents, err := os.ReadFile(CollectorCertificatePath(paths))
	if err != nil {
		return nil, fmt.Errorf("read pinned collector certificate: %w", err)
	}
	block, rest := pem.Decode(contents)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return nil, fmt.Errorf("pinned collector certificate is malformed; existing files were retained")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pinned collector certificate: %w", err)
	}
	if err := certificate.CheckSignatureFrom(certificate); err != nil {
		return nil, fmt.Errorf("collector certificate must be its self-signed installation identity: %w", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	if _, err := certificate.Verify(x509.VerifyOptions{Roots: roots, DNSName: "localhost", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		return nil, fmt.Errorf("pinned collector certificate is not valid: %w", err)
	}
	for _, loopback := range []string{"127.0.0.1", "::1"} {
		if err := certificate.VerifyHostname(loopback); err != nil {
			return nil, fmt.Errorf("pinned collector certificate does not cover loopback: %w", err)
		}
	}
	return certificate, nil
}

func CollectorServerTLS(paths Paths, listen string) (*tls.Config, error) {
	if err := EnsureCollectorTLS(paths, listen); err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(listen)
	identity, err := loadCollectorIdentity(paths, host)
	if err != nil {
		return nil, err
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{identity}}, nil
}

// CollectorClientTLS reads installation trust only; it never enrolls a receiver.
func CollectorClientTLS(paths Paths) (*tls.Config, error) {
	if _, err := os.Lstat(paths.Home); err != nil {
		return nil, fmt.Errorf("collector TLS trust is not installed: %w", err)
	}
	if err := EnsurePrivateHome(paths.Home); err != nil {
		return nil, err
	}
	identity, err := loadCollectorIdentity(paths, "localhost")
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(identity.Leaf)
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, nil
}
