package hash

import (
	"crypto/sha256"
	"fmt"
)

// Hash computes a hash from the given input string.
func Hash(config string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(config)))
}
