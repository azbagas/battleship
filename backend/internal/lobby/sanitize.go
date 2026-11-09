package lobby

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrInvalidDisplayName is returned when a display name does not meet length or character requirements.
var ErrInvalidDisplayName = errors.New("display name must be between 1 and 20 characters")

// SanitizeDisplayName strips ASCII and Unicode control/format characters, trims leading
// and trailing whitespace, and verifies that the resulting name contains between 1 and 20 runes.
func SanitizeDisplayName(raw string) (string, error) {
	var sb strings.Builder
	sb.Grow(len(raw))

	for _, r := range raw {
		// Strip ASCII controls (0x00-0x1F, 0x7F) and Unicode control (Cc) & format (Cf) characters.
		if r < 32 || r == 127 || unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		sb.WriteRune(r)
	}

	cleaned := strings.TrimSpace(sb.String())
	runeCount := utf8.RuneCountInString(cleaned)
	if runeCount < 1 || runeCount > 20 {
		return "", ErrInvalidDisplayName
	}

	return cleaned, nil
}

