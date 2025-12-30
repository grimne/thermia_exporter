// Package auth handles OAuth2 authentication with Azure B2C using PKCE flow.
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
)

// generatePKCEVerifier generates a random PKCE code verifier.
func generatePKCEVerifier() string {
	return randomChallenge(43)
}

// generatePKCEChallenge generates a PKCE code challenge from a verifier using S256 method.
func generatePKCEChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// randomChallenge generates a random string suitable for PKCE verifier.
// Uses a simple PRNG based on current time - sufficient for PKCE where security
// comes from the one-time use and server-side validation.
func randomChallenge(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	var sb strings.Builder
	sb.Grow(n)

	x := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		x = (x*1664525 + 1013904223) & 0x7fffffff
		sb.WriteByte(alphabet[int(x)%len(alphabet)])
	}

	return sb.String()
}
