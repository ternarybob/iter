package sdk

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var idCounter uint64

// generateID creates a unique identifier.
func generateID() string {
	// Use timestamp + random bytes + counter for uniqueness
	timestamp := time.Now().UnixNano()
	counter := atomic.AddUint64(&idCounter, 1)

	// Generate 4 random bytes
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)

	// Combine into ID
	id := make([]byte, 16)

	// Timestamp (8 bytes)
	for i := 7; i >= 0; i-- {
		id[i] = byte(timestamp)
		timestamp >>= 8
	}

	// Counter (4 bytes)
	for i := 11; i >= 8; i-- {
		id[i] = byte(counter)
		counter >>= 8
	}

	// Random (4 bytes)
	copy(id[12:], randomBytes)

	return hex.EncodeToString(id)
}

// GenerateID creates a unique identifier (exported version).
func GenerateID() string {
	return generateID()
}
