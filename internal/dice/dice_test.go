package dice

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestParseValid checks the notations the parser must accept: implicit
// count, signed modifiers, upper-case D, and stray whitespace.
func TestParseValid(t *testing.T) {
	cases := []struct {
		notation string
		want     Spec
	}{
		{"d20", Spec{Count: 1, Sides: 20}},
		{"1d20", Spec{Count: 1, Sides: 20}},
		{"2d6", Spec{Count: 2, Sides: 6}},
		{"2d6+3", Spec{Count: 2, Sides: 6, Modifier: 3}},
		{"2d6-1", Spec{Count: 2, Sides: 6, Modifier: -1}},
		{"4D6 + 1", Spec{Count: 4, Sides: 6, Modifier: 1}},
	}
	for _, tc := range cases {
		got, err := Parse(tc.notation)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", tc.notation, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.notation, got, tc.want)
		}
	}
}

// TestParseInvalid checks that every malformed or out-of-limit notation
// is refused with an error.
func TestParseInvalid(t *testing.T) {
	cases := []string{
		"",            // empty
		"banana",      // no d
		"d",           // no sides
		"0d6",         // zero dice
		"-2d6",        // negative dice
		"1.5d6",       // fractional dice
		"2d0",         // zero sides
		"2d6+three",   // wordy modifier
		"2d6++3",      // double sign
		"2d6 + 3d4",   // second dice group
		"101d6",       // over the dice limit
		"2d1001",      // over the sides limit
		"2d6+1000001", // over the modifier limit
	}
	for _, notation := range cases {
		if _, err := Parse(notation); err == nil {
			t.Errorf("Parse(%q) = no error, want one", notation)
		}
	}
}

// TestRollBounds verifies that a real throw stays between the smallest
// and the largest possible total, and that it can hit both ends.
func TestRollBounds(t *testing.T) {
	spec := Spec{Count: 2, Sides: 6, Modifier: 3} // 2d6+3: totals 5..15
	low, high := false, false
	for range 500 {
		got, err := spec.Roll()
		if err != nil {
			t.Fatalf("Roll() failed: %v", err)
		}
		if got < 5 || got > 15 {
			t.Fatalf("Roll() = %d, want a total in [5, 15]", got)
		}
		low, high = low || got == 5, high || got == 15
	}
	if !low || !high {
		t.Errorf("500 throws never hit an end: low=%v high=%v", low, high)
	}
}

// TestRollWithFixedReader feeds a fixed reader in place of crypto/rand
// and verifies the exact arithmetic: byte 0x00 is face 1, byte 0x05 is
// face 6, and the modifier lands on the total once.
func TestRollWithFixedReader(t *testing.T) {
	spec := Spec{Count: 2, Sides: 6, Modifier: 3}
	got, err := spec.roll(strings.NewReader("\x00\x05"))
	if err != nil {
		t.Fatalf("roll() failed: %v", err)
	}
	if want := 1 + 6 + 3; got != want {
		t.Errorf("roll(fixed) = %d, want %d", got, want)
	}
}

// TestRollReportsMissingRandomness verifies that a failed random source
// surfaces as an error instead of a silent total.
func TestRollReportsMissingRandomness(t *testing.T) {
	if _, err := (Spec{Count: 1, Sides: 6}).roll(strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Errorf("roll(empty source) error = %v, want io.EOF", err)
	}
}

// TestRollRejectsInvalidSpec verifies that a spec built by hand, not by
// Parse, still meets the package limits before any randomness is drawn.
func TestRollRejectsInvalidSpec(t *testing.T) {
	cases := []Spec{
		{Count: 0, Sides: 6},
		{Count: 2, Sides: 1},
		{Count: 2, Sides: 6, Modifier: maxModifier + 1},
	}
	for _, spec := range cases {
		if _, err := spec.Roll(); err == nil {
			t.Errorf("Roll(%+v) = no error, want one", spec)
		}
	}
}

// TestRollShortcut verifies the one-call path: a good notation throws,
// a bad one fails.
func TestRollShortcut(t *testing.T) {
	if _, err := Roll("3d20+1"); err != nil {
		t.Errorf("Roll(3d20+1) failed: %v", err)
	}
	if _, err := Roll("nope"); err == nil {
		t.Errorf("Roll(nope) = no error, want one")
	}
}
