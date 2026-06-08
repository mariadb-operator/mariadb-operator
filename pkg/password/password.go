package password

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	// length is the total number of characters in the generated password.
	length = 16

	// lowerChars, upperChars, digitChars, and symbolChars are the character
	// sets used for password generation and compliance validation. All sets are
	// ASCII-only; the generation logic relies on byte-level indexing and would
	// need to be updated to handle multi-byte runes.
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars  = "0123456789"
	symbolChars = "~!@%^&*()_+-={}|[]:<>/"

	// allChars is the pool used to fill remaining positions after the minimum
	// character class requirements have been satisfied.
	allChars = lowerChars + upperChars + digitChars + symbolChars
)

// charGroup pairs a character set with the minimum number of characters that
// must appear from that set in every generated password.
type charGroup struct {
	chars    string
	minCount int
}

// charGroups defines the guaranteed character composition of every generated
// password: at least one lowercase letter, one uppercase letter, four digits,
// and two symbols. The sum of minCount values is the effective minimum password
// length.
var charGroups = []charGroup{
	{lowerChars, 1},
	{upperChars, 1},
	{digitChars, 4},
	{symbolChars, 2},
}

// Generate returns a password that satisfies the MariaDB simple_password_check
// default policy. The password is 16 characters long with guaranteed
// representation from each required character class, filled to the target
// length from the full character pool, and shuffled with a
// cryptographically secure Fisher-Yates shuffle.
func Generate() (string, error) {
	pw := make([]byte, 0, length)

	// Satisfy per-class minimums first.
	for _, g := range charGroups {
		for range g.minCount {
			ch, err := randomChar(g.chars)
			if err != nil {
				return "", err
			}
			pw = append(pw, ch)
		}
	}

	// Fill remaining positions from the full character pool.
	for len(pw) < length {
		ch, err := randomChar(allChars)
		if err != nil {
			return "", err
		}
		pw = append(pw, ch)
	}

	if err := shuffle(pw); err != nil {
		return "", err
	}

	return string(pw), nil
}

// randomChar returns a cryptographically random character from chars.
func randomChar(chars string) (byte, error) {
	if len(chars) == 0 {
		return 0, fmt.Errorf("character set cannot be empty")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
	if err != nil {
		return 0, fmt.Errorf("error generating random index: %w", err)
	}
	return chars[n.Int64()], nil
}

// shuffle performs a cryptographically secure Fisher-Yates shuffle on items
// in-place.
func shuffle(items []byte) error {
	for i := len(items) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("error generating random index: %w", err)
		}
		j := int(n.Int64())
		items[i], items[j] = items[j], items[i]
	}
	return nil
}
