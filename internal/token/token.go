package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func GenerateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("could not generate token id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Sign computes HMAC-SHA256 over the token fields using the shared passphrase.
// Fields are joined with "|" as delimiter. All signed fields (tokenID, taskID,
// masterID) must be UUIDs and expiresAt must be RFC3339 - none of which
// can contain "|". Do not change field types without updating this function.
func Sign(tokenID, taskID, masterID string, expiresAt time.Time, passphrase string) string {
	message := strings.Join([]string{
		tokenID,
		taskID,
		masterID,
		expiresAt.UTC().Format(time.RFC3339),
	}, "|")

	mac := hmac.New(sha256.New, []byte(passphrase))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(tokenValue, tokenID, taskID, masterID string, expiresAt time.Time, passphrase string) bool {
	expected := Sign(tokenID, taskID, masterID, expiresAt, passphrase)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}

	tokenBytes, err := hex.DecodeString(tokenValue)
	if err != nil {
		return false
	}

	return hmac.Equal(expectedBytes, tokenBytes)
}

func IsExpired(expiresAt time.Time) bool {
	return !time.Now().UTC().Before(expiresAt.UTC())
}

func ComputeExpiry(from time.Time, maxDurationSeconds uint32) time.Time {
	return from.UTC().Add(time.Duration(maxDurationSeconds) * time.Second)
}
