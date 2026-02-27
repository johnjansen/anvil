package audit

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const keyFile = "audit-key"
const keySize = 32

// LoadOrCreateKey returns the 32-byte signing key for the project.
// If the key file does not exist, it generates one with crypto/rand.
func LoadOrCreateKey(projectPath string) ([]byte, error) {
	keyPath := filepath.Join(projectPath, ".anvil", keyFile)

	data, err := os.ReadFile(keyPath)
	if err == nil {
		key, err := hex.DecodeString(string(data))
		if err != nil {
			return nil, fmt.Errorf("decoding audit key: %w", err)
		}
		return key, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading audit key: %w", err)
	}

	// Generate new key
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating audit key: %w", err)
	}

	// Ensure .anvil directory exists
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return nil, fmt.Errorf("creating .anvil dir: %w", err)
	}

	// Write hex-encoded key with 0600 permissions
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600); err != nil {
		return nil, fmt.Errorf("writing audit key: %w", err)
	}

	return key, nil
}

// Sign computes an HMAC-SHA256 signature and returns it hex-encoded.
func Sign(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks that the hex-encoded signature matches the HMAC-SHA256 of data.
func Verify(key, data []byte, signature string) bool {
	expected := Sign(key, data)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SignRunRecord computes an HMAC signature for a RunRecord.
// It serializes the record JSON (without the signature field) and sets rec.Signature.
// The caller must pass the JSON bytes of the record with signature excluded.
func SignRunRecord(projectPath string, recJSON []byte) (string, error) {
	key, err := LoadOrCreateKey(projectPath)
	if err != nil {
		return "", err
	}
	return Sign(key, recJSON), nil
}

// VerifyRunRecord checks whether the signature matches the record content.
func VerifyRunRecord(projectPath string, recJSON []byte, signature string) (bool, error) {
	key, err := LoadOrCreateKey(projectPath)
	if err != nil {
		return false, err
	}
	return Verify(key, recJSON, signature), nil
}
