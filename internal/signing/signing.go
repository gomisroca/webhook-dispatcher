// Package signing provies HMAC-SHA256 signing for outgoing webhook deliveries
//
// X-Webhook-Signature: sha256=<hex_digest>
package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

// Returns the signature header value for payload signed with the given secret
func Sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%x", mac.Sum(nil))
}

// Checks a signature using constant-time comparison
func Verify(payload []byte, secret, signature string) bool {
	expected := Sign(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}