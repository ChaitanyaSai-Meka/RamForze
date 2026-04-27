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

var (
	hmacByteLen = sha256.Size
	hmacHexLen  = hex.EncodedLen(sha256.Size)
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

func computeHMAC(tokenID, taskID, masterID string, expiresAt time.Time, passphrase string) []byte {
	message := strings.Join([]string{
		tokenID,
		taskID,
		masterID,
		expiresAt.UTC().Format(time.RFC3339),
	}, "|")
	mac := hmac.New(sha256.New, []byte(passphrase))
	mac.Write([]byte(message))
	return mac.Sum(nil)
}

func validateFields(fields ...string) error {
	for _, f := range fields {
		if strings.Contains(f, "|") {
			return fmt.Errorf("signed field must not contain '|': %q", f)
		}
	}
	return nil
}

// Sign returns the hex-encoded HMAC signature for use on the wire.
// Fields are joined with "|" as delimiter. The string fields (tokenID,
// taskID, masterID) must therefore not contain "|". expiresAt is formatted
// in UTC using RFC3339 when computing the MAC. Do not change field types or
// serialization without updating this function.
func Sign(tokenID, taskID, masterID string, expiresAt time.Time, passphrase string) (string, error) {
	if err := validateFields(tokenID, taskID, masterID); err != nil {
		return "", err
	}
	return hex.EncodeToString(computeHMAC(tokenID, taskID, masterID, expiresAt, passphrase)), nil
}

func Verify(tokenValue, tokenID, taskID, masterID string, expiresAt time.Time, passphrase string) bool {
	if len(tokenValue) != hmacHexLen {
		return false
	}
	tokenBytes, err := hex.DecodeString(tokenValue)
	if err != nil || len(tokenBytes) != hmacByteLen {
		return false
	}
	if err := validateFields(tokenID, taskID, masterID); err != nil {
		return false
	}
	expected := computeHMAC(tokenID, taskID, masterID, expiresAt, passphrase)
	return hmac.Equal(expected, tokenBytes)
}

func IsExpired(expiresAt time.Time) bool {
	return !time.Now().UTC().Before(expiresAt.UTC())
}

func ComputeExpiry(from time.Time, maxDurationSeconds uint32) time.Time {
	return from.UTC().Add(time.Duration(maxDurationSeconds) * time.Second)
}
