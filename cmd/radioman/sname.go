package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var specialRuneMap = map[rune]string{
	'ı': "i",
	'İ': "I",
	'ğ': "g",
	'Ğ': "G",
	'ş': "s",
	'Ş': "S",
	'ç': "c",
	'Ç': "C",
	'ö': "o",
	'Ö': "O",
	'ü': "u",
	'Ü': "U",
	'ß': "ss",
	'ø': "o",
	'Ø': "O",
	'ł': "l",
	'Ł': "L",
	'đ': "d",
	'Đ': "D",
	'æ': "ae",
	'Æ': "AE",
	'œ': "oe",
	'Œ': "OE",
	'ð': "d",
	'Ð': "D",
	'þ': "th",
	'Þ': "TH",
}

// IsValidShortNameRune reports whether r is allowed in a short name (a-zA-Z0-9-_.).
func IsValidShortNameRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.'
}

// ValidateShortName verifies that s is a non-empty valid short name.
func ValidateShortName(s string) error {
	if s == "" {
		return errors.New("short name cannot be empty")
	}
	for _, r := range s {
		if !IsValidShortNameRune(r) {
			return fmt.Errorf("invalid character %q in short name %q (allowed: a-z, A-Z, 0-9, '-', '_', '.')", r, s)
		}
	}
	return nil
}

// GenerateShortName derives a clean short name from a station title.
func GenerateShortName(title string) string {
	words := strings.Fields(title)
	cleanedWords := make([]string, 0, len(words))

	for _, word := range words {
		var sb strings.Builder
		for _, r := range word {
			if IsValidShortNameRune(r) {
				sb.WriteRune(r)
				continue
			}
			if mapped, ok := specialRuneMap[r]; ok {
				sb.WriteString(mapped)
				continue
			}
			decomposed := norm.NFD.String(string(r))
			for _, nr := range decomposed {
				if unicode.Is(unicode.Mn, nr) {
					continue
				}
				if (nr >= 'a' && nr <= 'z') || (nr >= 'A' && nr <= 'Z') || (nr >= '0' && nr <= '9') {
					sb.WriteRune(nr)
				}
			}
		}

		w := collapseDashes(sb.String())
		w = strings.Trim(w, "-_.")
		if w != "" {
			cleanedWords = append(cleanedWords, w)
		}
	}

	joined := collapseDashes(strings.Join(cleanedWords, "-"))
	joined = strings.Trim(joined, "-")
	if joined == "" {
		return "station"
	}
	return joined
}

// GenerateUniqueShortName generates a short name from title that does not collide
// (case-insensitively) with any name in existing.
func GenerateUniqueShortName(title string, existing []string) string {
	used := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		if name != "" {
			used[strings.ToLower(name)] = struct{}{}
		}
	}

	base := GenerateShortName(title)
	if _, exists := used[strings.ToLower(base)]; !exists {
		return base
	}

	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if _, exists := used[strings.ToLower(candidate)]; !exists {
			return candidate
		}
	}
}

func collapseDashes(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if prevDash {
				continue
			}
			prevDash = true
		} else {
			prevDash = false
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
