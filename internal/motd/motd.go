// Package motd serves the Kit message of the day: one short line of
// encouragement, chosen deterministically from the calendar date.
//
// The package is purely decorative. The message changes only when the
// calendar day changes, so repeated calls on the same day return the
// same words.
package motd

import "time"

// daysPerLeapYear is the number of days in a leap year. It shifts the
// day index between years so the sequence continues past New Year.
const daysPerLeapYear = 366

// messages holds the daily lines. Keep each entry short and kind.
var messages = []string{
	"Small steps, clean diffs.",
	"Read the error. It is trying to help.",
	"One test today is worth ten tomorrow.",
	"The bug is a door. Walk through it.",
	"Name things well; future you says thanks.",
	"Refactor with care, commit with courage.",
	"Your code compiles. Cherish this moment.",
	"Rest is part of the build pipeline.",
	"Simplify until it almost hurts.",
	"Run go test, and believe.",
}

// Today returns the message for the current day, using the local clock.
func Today() string {
	return At(time.Now())
}

// At returns the message for the calendar day of now.
//
// The choice is a pure function of the day: the same day always maps
// to the same message, and the sequence cycles through the whole set.
// Callers can pass any clock they like, which keeps the function easy
// to test.
func At(now time.Time) string {
	if len(messages) == 0 {
		return ""
	}
	day := now.YearDay() + daysPerLeapYear*now.Year()
	return messages[day%len(messages)]
}
