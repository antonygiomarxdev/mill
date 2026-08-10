package domain

import (
	"crypto/rand"
	"encoding/hex"
)

// randomSuffix generates a short random hex string for unique IDs.
func randomSuffix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
