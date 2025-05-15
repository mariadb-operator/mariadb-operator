package hash

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Hash computes a hash from the given input string.
func Hash(v string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(v)))
}

// HashJSON computes the hash from a JSON object.
func HashJSON(v any) (string, error) {
	// IMPORTANT: Map values are sorted in order to obtain a canonical representation.
	// From the json.Marshal docs: https://pkg.go.dev/encoding/json#Marshal
	// Map values encode as JSON objects. The map's key type must either be a string, an integer type, or implement encoding.TextMarshaler.
	// The map keys are sorted and used as JSON object keys by applying the following rules, subject to the UTF-8 coercion described
	// for string values above:
	// - keys of any string type are used directly
	// - keys that implement encoding.TextMarshaler are marshaled
	// - integer keys are converted to strings
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("error mashaling JSON: %v", err)
	}
	return Hash(string(bytes)), nil
}
