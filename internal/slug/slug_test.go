package slug

import "testing"

func TestMake(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "simple words", text: "Hello World", want: "hello-world"},
		{name: "collapse whitespace", text: "  multiple   spaces  ", want: "multiple-spaces"},
		{name: "punctuation becomes separator", text: "Hello, World!", want: "hello-world"},
		{name: "keeps digits", text: "Release 2 and 10", want: "release-2-and-10"},
		{name: "empty text", text: "", want: ""},
		{name: "no letters or digits", text: " ... --- ... ", want: ""},
		{name: "single word", text: "Solo", want: "solo"},
		{name: "non-ascii letters", text: "Café Ünïcode", want: "café-ünïcode"},
		{name: "non-ascii uppercase folds", text: "Δ Ο Δ", want: "δ-ο-δ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Make(tt.text); got != tt.want {
				t.Errorf("Make(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestMakeWith(t *testing.T) {
	tests := []struct {
		name string
		text string
		sep  rune
		want string
	}{
		{name: "underscore separator", text: "Hello World", sep: '_', want: "hello_world"},
		{name: "dot separator", text: "a.b..c", sep: '.', want: "a.b.c"},
		{name: "custom punctuation", text: "one,two three", sep: '+', want: "one+two+three"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeWith(tt.text, tt.sep); got != tt.want {
				t.Errorf("MakeWith(%q, %q) = %q, want %q", tt.text, tt.sep, got, tt.want)
			}
		})
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name string
		text string
		sep  rune
		want bool
	}{
		{name: "valid slug", text: "hello-world", sep: '-', want: true},
		{name: "digits only", text: "123", sep: '-', want: true},
		{name: "leading separator", text: "-hello", sep: '-', want: false},
		{name: "trailing separator", text: "hello-", sep: '-', want: false},
		{name: "double separator", text: "hello--world", sep: '-', want: false},
		{name: "uppercase rejected", text: "Hello", sep: '-', want: false},
		{name: "empty rejected", text: "", sep: '-', want: false},
		{name: "space rejected", text: "hello world", sep: '-', want: false},
		{name: "wrong separator rejected", text: "hello_world", sep: '-', want: false},
		{name: "separator only", text: "---", sep: '-', want: false},
		{name: "custom separator ok", text: "hello_world", sep: '_', want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.text, tt.sep); got != tt.want {
				t.Errorf("Is(%q, %q) = %v, want %v", tt.text, tt.sep, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		slug string
		max  int
		sep  rune
		want string
	}{
		{name: "fits already", slug: "hello-world", max: 20, sep: '-', want: "hello-world"},
		{name: "exact fit", slug: "hello-world", max: 11, sep: '-', want: "hello-world"},
		{name: "cut on separator", slug: "alpha-beta-gamma", max: 10, sep: '-', want: "alpha"},
		{name: "no separator in range", slug: "helloworld", max: 5, sep: '-', want: "hello"},
		{name: "zero max", slug: "hello-world", max: 0, sep: '-', want: ""},
		{name: "negative max", slug: "hello-world", max: -3, sep: '-', want: ""},
		{name: "empty slug", slug: "", max: 5, sep: '-', want: ""},
		{name: "custom separator", slug: "alpha_beta_gamma", max: 10, sep: '_', want: "alpha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.slug, tt.max, tt.sep); got != tt.want {
				t.Errorf("Truncate(%q, %d, %q) = %q, want %q", tt.slug, tt.max, tt.sep, got, tt.want)
			}
		})
	}
}

// TestRoundTripIsStable checks the property that makes the package useful:
// a slug made by Make always passes Is, and re-making a slug does not
// change it.
func TestRoundTripIsStable(t *testing.T) {
	inputs := []string{
		"Hello World",
		"  tab\tseparated\tvalues  ",
		"Mixed_underscores-and-dashes",
		"Δοκουμέντο 42!",
	}

	for _, in := range inputs {
		s := Make(in)
		if !Is(s, '-') {
			t.Errorf("Make(%q) = %q, which Is reports as invalid", in, s)
		}
		if again := Make(s); again != s {
			t.Errorf("Make(%q) = %q, but re-making gave %q", s, s, again)
		}
	}
}
