package config

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTLSCollectorRejectsPreviouslyExposedV1Token(t *testing.T) {
	paths, _ := Resolve(t.TempDir())
	key, err := PromptHashKey(paths)
	if err != nil {
		t.Fatal(err)
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte("qlog-collector-auth-v1"))
	oldToken := hex.EncodeToString(digest.Sum(nil))
	current, err := CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	if current == oldToken {
		t.Fatal("previously exposed v1 credential is still active")
	}
}

func TestTLSIdentityPersistsAcrossRestartsAndConcurrentCreation(t *testing.T) {
	paths, _ := Resolve(t.TempDir())
	var group sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			errors <- EnsureCollectorTLS(paths, "127.0.0.1:4318")
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	certBefore, err := os.ReadFile(CollectorCertificatePath(paths))
	if err != nil {
		t.Fatal(err)
	}
	keyBefore, err := os.ReadFile(collectorPrivateKeyPath(paths))
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := CollectorServerTLS(paths, "localhost:4318"); err != nil {
			t.Fatal(err)
		}
		if _, err := CollectorClientTLS(paths); err != nil {
			t.Fatal(err)
		}
	}
	certAfter, _ := os.ReadFile(CollectorCertificatePath(paths))
	keyAfter, _ := os.ReadFile(collectorPrivateKeyPath(paths))
	if !bytes.Equal(certBefore, certAfter) || !bytes.Equal(keyBefore, keyAfter) {
		t.Fatal("restart silently rotated identity")
	}
	entries, err := os.ReadDir(paths.Home)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "collector-tls" {
			t.Fatalf("concurrent publication left staging data: %s", entry.Name())
		}
	}
}

func TestTLSClientNeverCreatesOrEnrollsAnIdentity(t *testing.T) {
	paths, _ := Resolve(filepath.Join(t.TempDir(), "missing-home"))
	if _, err := CollectorClientTLS(paths); err == nil {
		t.Fatal("missing trust was accepted")
	}
	if _, err := os.Stat(paths.Home); !os.IsNotExist(err) {
		t.Fatal("client created a home")
	}
	if err := EnsurePrivateHome(paths.Home); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectorClientTLS(paths); err == nil {
		t.Fatal("uninstalled trust was accepted")
	}
	if _, err := os.Stat(filepath.Join(paths.Home, "collector-tls")); !os.IsNotExist(err) {
		t.Fatal("client enrolled an identity")
	}
}

func TestTLSInvalidExistingIdentitiesFailClosedWithoutRewriting(t *testing.T) {
	for _, kind := range []string{"missing key", "malformed cert", "malformed key", "mismatched key", "expired cert"} {
		t.Run(kind, func(t *testing.T) {
			paths, _ := Resolve(t.TempDir())
			server, err := CollectorServerTLS(paths, "127.0.0.1:4318")
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "missing key":
				if err := os.Remove(collectorPrivateKeyPath(paths)); err != nil {
					t.Fatal(err)
				}
			case "malformed cert":
				if err := os.WriteFile(CollectorCertificatePath(paths), []byte("MOCK_INVALID_CERT"), 0600); err != nil {
					t.Fatal(err)
				}
			case "malformed key":
				if err := os.WriteFile(collectorPrivateKeyPath(paths), []byte("MOCK_INVALID_KEY"), 0600); err != nil {
					t.Fatal(err)
				}
			case "mismatched key":
				other, _ := Resolve(t.TempDir())
				if err := EnsureCollectorTLS(other, "127.0.0.1:4318"); err != nil {
					t.Fatal(err)
				}
				key, err := os.ReadFile(collectorPrivateKeyPath(other))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(collectorPrivateKeyPath(paths), key, 0600); err != nil {
					t.Fatal(err)
				}
			case "expired cert":
				expired := *server.Certificates[0].Leaf
				expired.NotBefore = time.Now().Add(-48 * time.Hour)
				expired.NotAfter = time.Now().Add(-24 * time.Hour)
				der, err := x509.CreateCertificate(rand.Reader, &expired, &expired, expired.PublicKey, server.Certificates[0].PrivateKey)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(CollectorCertificatePath(paths), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
					t.Fatal(err)
				}
			}
			certBefore, _ := os.ReadFile(CollectorCertificatePath(paths))
			keyBefore, _ := os.ReadFile(collectorPrivateKeyPath(paths))
			if err := EnsureCollectorTLS(paths, "127.0.0.1:4318"); err == nil {
				t.Fatal("invalid server identity accepted")
			}
			if _, err := CollectorClientTLS(paths); err == nil {
				t.Fatal("invalid client trust identity accepted")
			}
			certAfter, _ := os.ReadFile(CollectorCertificatePath(paths))
			keyAfter, _ := os.ReadFile(collectorPrivateKeyPath(paths))
			if !bytes.Equal(certBefore, certAfter) || !bytes.Equal(keyBefore, keyAfter) {
				t.Fatal("invalid identity was silently rewritten")
			}
		})
	}
}

func TestTLSConcreteNonLoopbackSANMustMatchWithoutSilentRotation(t *testing.T) {
	paths, _ := Resolve(t.TempDir())
	if err := EnsureCollectorTLS(paths, "127.0.0.1:4318"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(CollectorCertificatePath(paths))
	if err := EnsureCollectorTLS(paths, "192.0.2.1:4318"); err == nil {
		t.Fatal("existing identity unexpectedly covers new listener")
	}
	after, _ := os.ReadFile(CollectorCertificatePath(paths))
	if !bytes.Equal(before, after) {
		t.Fatal("SAN mismatch silently rotated trust")
	}
	other, _ := Resolve(t.TempDir())
	server, err := CollectorServerTLS(other, "192.0.2.1:4318")
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Certificates[0].Leaf.VerifyHostname("192.0.2.1"); err != nil {
		t.Fatal(err)
	}
}
