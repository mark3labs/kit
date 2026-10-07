package roman

import "testing"

func TestToRoman(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{name: "one", n: 1, want: "I"},
		{name: "three", n: 3, want: "III"},
		{name: "four subtractive", n: 4, want: "IV"},
		{name: "nine subtractive", n: 9, want: "IX"},
		{name: "fourteen", n: 14, want: "XIV"},
		{name: "forty", n: 40, want: "XL"},
		{name: "ninety", n: 90, want: "XC"},
		{name: "four hundred", n: 400, want: "CD"},
		{name: "nine hundred", n: 900, want: "CM"},
		{name: "mixed subtractives", n: 1994, want: "MCMXCIV"},
		{name: "current year", n: 2026, want: "MMXXVI"},
		{name: "largest", n: 3999, want: "MMMCMXCIX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToRoman(tt.n)
			if err != nil {
				t.Fatalf("ToRoman(%d) returned error: %v", tt.n, err)
			}
			if got != tt.want {
				t.Errorf("ToRoman(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestToRomanErrors(t *testing.T) {
	for _, n := range []int{-1, 0, 4000, 10000} {
		if got, err := ToRoman(n); err == nil {
			t.Errorf("ToRoman(%d) = %q, want error", n, got)
		}
	}
}

func TestFromRoman(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "single I", input: "I", want: 1},
		{name: "additive", input: "VIII", want: 8},
		{name: "subtractive pairs", input: "MCMXCIV", want: 1994},
		{name: "mixed", input: "MMMCMXCIX", want: 3999},
		{name: "four fifties deep", input: "MMXXVI", want: 2026},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromRoman(tt.input)
			if err != nil {
				t.Fatalf("FromRoman(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("FromRoman(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromRomanErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "over repeats a symbol", input: "IIII"},
		{name: "repeats a five-symbol", input: "VV"},
		{name: "illegal subtraction", input: "IL"},
		{name: "beyond range", input: "MMMM"},
		{name: "lowercase", input: "iv"},
		{name: "contains a space", input: "X I"},
		{name: "not canonical", input: "VIV"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := FromRoman(tt.input); err == nil {
				t.Errorf("FromRoman(%q) = %d, want error", tt.input, got)
			}
		})
	}
}

// TestRoundTrip checks that ToRoman and FromRoman invert each other over the
// full supported range.
func TestRoundTrip(t *testing.T) {
	for n := 1; n <= 3999; n++ {
		s, err := ToRoman(n)
		if err != nil {
			t.Fatalf("ToRoman(%d) returned error: %v", n, err)
		}
		got, err := FromRoman(s)
		if err != nil {
			t.Fatalf("FromRoman(%q) returned error: %v", s, err)
		}
		if got != n {
			t.Fatalf("round trip of %d: FromRoman(ToRoman(%d)) = %d", n, n, got)
		}
	}
}
