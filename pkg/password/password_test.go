package password

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	for i := 0; i < 1000; i++ {
		pw, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}

		if len(pw) != length {
			t.Errorf("password length = %d, want %d", len(pw), length)
		}

		if !isCompliant(pw) {
			t.Errorf("password not compliant: %q", pw)
		}
	}
}

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		pw, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}

		if _, ok := seen[pw]; ok {
			t.Fatalf("duplicate password after %d iterations: %q", i, pw)
		}
		seen[pw] = struct{}{}
	}
}

func TestIsCompliant(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want bool
	}{
		{"compliant", "abcDE1234!@", true},
		{"missing uppercase", "abcdef1234!@", false},
		{"missing lowercase", "ABCDEF1234!@", false},
		{"missing digit", "abcDEFghij!@", false},
		{"missing symbol", "abcDEFghij1234", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCompliant(tt.pw); got != tt.want {
				t.Errorf("isCompliant(%q) = %v, want %v", tt.pw, got, tt.want)
			}
		})
	}
}

// isCompliant reports whether pw satisfies the MariaDB simple_password_check
// default policy: at least one uppercase letter, one lowercase letter, one
// digit, and one special character from symbolChars.
//
// Compliance is guaranteed by construction in generatePassword; this function
// exists for use in tests and defensive assertions.
func isCompliant(pw string) bool {
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, c := range pw {
		switch {
		case strings.ContainsRune(upperChars, c):
			hasUpper = true
		case strings.ContainsRune(lowerChars, c):
			hasLower = true
		case strings.ContainsRune(digitChars, c):
			hasDigit = true
		case strings.ContainsRune(symbolChars, c):
			hasSymbol = true
		}
		if hasUpper && hasLower && hasDigit && hasSymbol {
			return true
		}
	}
	return false
}
