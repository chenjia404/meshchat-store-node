package p2p

import (
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func TestLoadOrCreateIdentityKeyPersists(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "node_identity.key")

	firstKey, err := LoadOrCreateIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentityKey(first) error = %v", err)
	}
	secondKey, err := LoadOrCreateIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateIdentityKey(second) error = %v", err)
	}

	firstBytes, err := crypto.MarshalPrivateKey(firstKey)
	if err != nil {
		t.Fatalf("MarshalPrivateKey(first) error = %v", err)
	}
	secondBytes, err := crypto.MarshalPrivateKey(secondKey)
	if err != nil {
		t.Fatalf("MarshalPrivateKey(second) error = %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("private key changed after reload")
	}
}
