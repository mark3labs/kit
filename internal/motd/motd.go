// Package motd serves the Kit message of the day: one short line of
// encouragement, chosen deterministically from the calendar date.
//
// The package is purely decorative. The message changes only when the
// calendar day changes, so repeated calls on the same day return the
// same words.
package motd

import "time"

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
//
// It is a thin wrapper around At(time.Now()).
func Today() string {
	return At(time.Now())
}

// At returns the message for the calendar day of now, in now's location.
//
// The choice is a pure function of the day: the same day always maps to
// the same message, consecutive days always move one step through the
// set (also across New Year), and the sequence repeats after
// len(messages) days. Callers can pass any clock they like, which keeps
// the function easy to test.
func At(now time.Time) string {
	n := len(messages)
	if n == 0 {
		return ""
	}
	day := dayIndex(now) % n
	if day < 0 {
		day += n
	}
	return messages[day]
}

// dayIndex counts the whole days between the Unix epoch and the midnight
// that starts now's calendar day, in now's location.
//
// Counting from local midnight (instead of the year number and the day
// of year) keeps the step between consecutive days at exactly one, no
// matter how long the year is or when daylight saving time shifts.
func dayIndex(now time.Time) int {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return int(midnight.Sub(time.Unix(0, 0)) / (24 * time.Hour))
}
