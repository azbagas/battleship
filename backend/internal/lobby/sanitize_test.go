package lobby

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:      "simple valid name",
			input:     "Alice",
			expected:  "Alice",
			expectErr: false,
		},
		{
			name:      "leading and trailing spaces trimmed",
			input:     "   Bob   ",
			expected:  "Bob",
			expectErr: false,
		},
		{
			name:      "embedded space preserved",
			input:     "Rear Admiral",
			expected:  "Rear Admiral",
			expectErr: false,
		},
		{
			name:      "strips ASCII control characters",
			input:     "\x00\x07Captain\x1f\x7f\r\n\t",
			expected:  "Captain",
			expectErr: false,
		},
		{
			name:      "strips Unicode format and control characters",
			input:     "\u200BZero\uFEFFWidth\u0085",
			expected:  "ZeroWidth",
			expectErr: false,
		},
		{
			name:      "empty input",
			input:     "",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "whitespace only input",
			input:     "    \t\r\n   ",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "control characters only input",
			input:     "\x00\x1f\x7f\u200b",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "exactly 1 rune",
			input:     "X",
			expected:  "X",
			expectErr: false,
		},
		{
			name:      "exactly 20 runes",
			input:     "12345678901234567890",
			expected:  "12345678901234567890",
			expectErr: false,
		},
		{
			name:      "21 runes fails",
			input:     "123456789012345678901",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "multibyte unicode runes counted properly (20 runes)",
			input:     "あいうえおかきくけこさしすせそたちつてと",
			expected:  "あいうえおかきくけこさしすせそたちつてと",
			expectErr: false,
		},
		{
			name:      "multibyte unicode runes exceeding 20 runes fails",
			input:     "あいうえおかきくけこさしすせそたちつてとな",
			expected:  "",
			expectErr: true,
		},
		{
			name:      "emoji runes counted by rune not byte length",
			input:     "Admiral 🚢",
			expected:  "Admiral 🚢",
			expectErr: false,
		},
		{
			name:      "stripping control chars reduces length to within 20",
			input:     strings.Repeat("\x00", 50) + "ValidName",
			expected:  "ValidName",
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := SanitizeDisplayName(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil (result: %q)", tc.input, res)
				}
				if !errors.Is(err, ErrInvalidDisplayName) {
					t.Fatalf("expected ErrInvalidDisplayName, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %q: %v", tc.input, err)
				}
				if res != tc.expected {
					t.Fatalf("expected %q, got %q", tc.expected, res)
				}
			}
		})
	}
}

