package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func CalculateExpiration(rememberMe bool) time.Time {
	if rememberMe {
		return time.Now().Add(30 * 24 * time.Hour)
	}
	return time.Now().Add(24 * time.Hour)
}
