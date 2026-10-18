package privacy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewSuppressionCleanupWorkerID returns a unique UUID for one worker process
// lifetime. A restart must use a new identity; row generations fence takeovers.
func NewSuppressionCleanupWorkerID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:]), nil
}
