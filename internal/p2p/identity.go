package p2p

import (
	"fmt"
	"os"
	"path/filepath"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

func LoadOrCreateIdentityKey(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		privKey, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("unmarshal private key: %w", err)
		}
		return privKey, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create identity key dir: %w", err)
	}

	privKey, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	encoded, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}
	return privKey, nil
}
