// Package roman converts between integers and Roman numerals.
//
// ToRoman renders integers in the canonical modern style with subtractive
// notation (IV, IX, XL, XC, CD, CM). FromRoman parses such numerals back and
// accepts only canonical input, so every string FromRoman accepts is exactly
// what ToRoman produces for the same value.
package roman

import (
	"fmt"
	"strings"
)

// valueSymbols lists numeral values in descending order. The subtractive
// pairs (900, 400, 90, 40, 9, 4) sit between their decimal neighbours so a
// single descending pass builds a canonical numeral.
var valueSymbols = []struct {
	value  int
	symbol string
}{
	{1000, "M"},
	{900, "CM"},
	{500, "D"},
	{400, "CD"},
	{100, "C"},
	{90, "XC"},
	{50, "L"},
	{40, "XL"},
	{10, "X"},
	{9, "IX"},
	{5, "V"},
	{4, "IV"},
	{1, "I"},
}

// maxRoman is the largest value representable without repeating M beyond the
// conventional limit of three.
const maxRoman = 3999

// validRunes is the full set of characters a Roman numeral may contain.
const validRunes = "IVXLCDM"

// ToRoman converts n to its canonical Roman numeral form.
//
// It returns an error when n falls outside the range [1, 3999], the largest
// value the standard symbol set represents without a vinculum (overline).
func ToRoman(n int) (string, error) {
	if n < 1 || n > maxRoman {
		return "", fmt.Errorf("roman: %d out of range [1, %d]", n, maxRoman)
	}
	var b strings.Builder
	for _, vs := range valueSymbols {
		for n >= vs.value {
			b.WriteString(vs.symbol)
			n -= vs.value
		}
	}
	return b.String(), nil
}

// FromRoman parses the Roman numeral s and returns its value.
//
// Parsing is strict: s must be a canonical numeral, meaning it must contain
// only the characters IVXLCDM and must be exactly the numeral ToRoman
// returns for its value. Non-canonical spellings such as IIII, VV or IL are
// rejected, as are empty strings, lowercase input and values above 3999.
func FromRoman(s string) (int, error) {
	for _, r := range s {
		if !strings.ContainsRune(validRunes, r) {
			return 0, fmt.Errorf("roman: invalid character %q in %q", r, s)
		}
	}
	orig := s
	n := 0
	for _, vs := range valueSymbols {
		for strings.HasPrefix(s, vs.symbol) {
			n += vs.value
			s = s[len(vs.symbol):]
		}
	}
	if s != "" {
		return 0, fmt.Errorf("roman: %q is not a valid numeral", orig)
	}
	if n < 1 || n > maxRoman {
		return 0, fmt.Errorf("roman: value %d out of range [1, %d]", n, maxRoman)
	}
	canonical, err := ToRoman(n)
	if err != nil {
		return 0, fmt.Errorf("roman: parsing %q: %w", orig, err)
	}
	if canonical != orig {
		return 0, fmt.Errorf("roman: %q is not canonical; use %q", orig, canonical)
	}
	return n, nil
}
