package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// CollectorToken authenticates legacy ingestion without exposing the prompt key.
func CollectorToken(paths Paths) (string, error) {
	if err := EnsurePrivateHome(paths.Home); err != nil {
		return "", err
	}
	key, err := PromptHashKey(paths)
	if err != nil {
		return "", err
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte("qlog-collector-auth-v2"))
	return hex.EncodeToString(digest.Sum(nil)), nil
}
