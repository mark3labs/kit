// Package dice rolls polyhedral dice for Kit easter eggs and demos.
//
// It speaks the classic tabletop notation "2d6+3": throw two six-sided
// dice and add three to the total. Rolls draw randomness from
// crypto/rand, so a throw is fit for a giveaway as well as for a game
// night. The package is purely decorative; nothing in Kit depends on
// it yet.
package dice

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
)

// Upper bounds on the parts of a throw. They keep one throw cheap to
// compute and its total far away from int overflow, even at the
// extremes.
const (
	maxDice     = 100       // dice in one throw
	maxSides    = 1000      // faces per die; covers d2 up to d1000
	maxModifier = 1_000_000 // flat add or subtract
)

// Spec describes one throw of the dice: count dice, each with sides
// faces, plus a flat modifier added once to the total.
type Spec struct {
	Count    int // number of dice; valid values are 1 to maxDice
	Sides    int // faces per die; valid values are 2 to maxSides
	Modifier int // flat bonus or penalty applied once; |modifier| ≤ maxModifier
}

// Parse reads dice notation such as "d20", "2d6", or "4d6-1" and
// returns the throw it names.
//
// An empty dice count means one die, so "d20" and "1d20" name the same
// throw. The d is case-insensitive, and whitespace is ignored, so
// "4D6 + 1" also reads fine.
func Parse(notation string) (Spec, error) {
	// Ignore all whitespace so "4d6 + 1" reads like "4d6+1".
	s := strings.ToLower(strings.Join(strings.Fields(notation), ""))

	countPart, rest, found := strings.Cut(s, "d")
	if !found {
		return Spec{}, fmt.Errorf("dice: %q has no d; want notation like 2d6+3", notation)
	}

	count := 1
	if countPart != "" {
		parsed, err := strconv.Atoi(countPart)
		switch {
		case err != nil:
			return Spec{}, fmt.Errorf("dice: dice count in %q must be a whole number: %w", notation, err)
		case parsed <= 0:
			return Spec{}, fmt.Errorf("dice: dice count in %q must be positive, got %d", notation, parsed)
		}
		count = parsed
	}

	// The rest now looks like "6", "6+3", or "6-1".
	modifier := 0
	if at := strings.IndexAny(rest, "+-"); at >= 0 {
		parsed, err := strconv.Atoi(rest[at:])
		if err != nil {
			return Spec{}, fmt.Errorf("dice: modifier in %q must be a whole number: %w", notation, err)
		}
		modifier = parsed
		rest = rest[:at]
	}

	sides, err := strconv.Atoi(rest)
	switch {
	case err != nil:
		return Spec{}, fmt.Errorf("dice: side count in %q must be a whole number: %w", notation, err)
	case sides <= 0:
		return Spec{}, fmt.Errorf("dice: side count in %q must be positive, got %d", notation, sides)
	}

	spec := Spec{Count: count, Sides: sides, Modifier: modifier}
	if err := spec.validate(); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// Roll throws the dice named by notation (see Parse) and returns the
// total. It is the one-call shortcut: dice.Roll("2d6+3").
func Roll(notation string) (int, error) {
	spec, err := Parse(notation)
	if err != nil {
		return 0, err
	}
	return spec.Roll()
}

// Roll throws the dice described by the spec and returns the total:
// the sum of all faces plus the modifier. Randomness comes from
// crypto/rand, the secure source of the operating system.
func (s Spec) Roll() (int, error) {
	return s.roll(rand.Reader)
}

// roll throws the dice, drawing one random number per die from random.
// It exists so tests can feed a fixed reader and predict every face;
// Roll uses crypto/rand.
func (s Spec) roll(random io.Reader) (int, error) {
	if err := s.validate(); err != nil {
		return 0, err
	}

	total := s.Modifier
	for range s.Count {
		face, err := rand.Int(random, big.NewInt(int64(s.Sides)))
		if err != nil {
			return 0, fmt.Errorf("dice: reading randomness: %w", err)
		}
		total += int(face.Int64()) + 1
	}
	return total, nil
}

// validate reports whether the spec lies within the package limits:
// one to maxDice dice, each with 2 to maxSides faces, and a modifier
// no larger in magnitude than maxModifier. The limits keep one throw
// cheap to compute and its total far away from int overflow.
func (s Spec) validate() error {
	switch {
	case s.Count < 1 || s.Count > maxDice:
		return fmt.Errorf("dice: dice count %d out of range [1, %d]", s.Count, maxDice)
	case s.Sides < 2 || s.Sides > maxSides:
		return fmt.Errorf("dice: side count %d out of range [2, %d]", s.Sides, maxSides)
	case s.Modifier < -maxModifier || s.Modifier > maxModifier:
		return fmt.Errorf("dice: modifier %d out of range [-%d, %d]", s.Modifier, maxModifier, maxModifier)
	}
	return nil
}
