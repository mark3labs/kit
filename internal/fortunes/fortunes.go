// Package fortunes serves fortune-cookie wisdom for the coding agent.
//
// Purely decorative: whenever the agent (or a human) needs a moment of
// groundless optimism, this package provides it. No tool integration, no
// lifecycle events, no cookies were harmed.
package fortunes

import "math/rand/v2"

// cookies holds the wisdom. Keep entries short, dry, and only mildly fatalistic.
var cookies = []string{
	"Your build will pass. Eventually. Probably.",
	"A nil pointer you encounter today has already been forgiven.",
	"The race condition you fear does not exist — it is a data race, and it does.",
	"rm -rf is only dangerous if you were not backing up. You were backing up. Right?",
	"Today is a good day to read the error message.",
	"The bug is not in your code. The bug is in your assumptions about your code.",
	"Someone, somewhere, is reading your stack trace and nodding knowingly.",
	"Your TODO comments are forming a small, patient civilization.",
	"git blame will reveal that it was you. It is always you. Be kind to that person.",
	"The dependency you refuse to vendor will outlive us all.",
	"Write tests. The fortune cookie cannot run them for you.",
	"Somewhere a flaky test just passed. Take it as a good omen.",
}

// All returns every fortune in the deck, in serving order.
//
// Callers that only want one cookie should use Random; All is for menus,
// teasers, and unit tests that check the kitchen.
func All() []string {
	out := make([]string, len(cookies))
	copy(out, cookies)
	return out
}

// Random returns one fortune, chosen by fate (crypto-grade unnecessary).
func Random() string {
	if len(cookies) == 0 {
		return "The cookie jar is empty. That, too, is a fortune."
	}
	return cookies[rand.IntN(len(cookies))]
}
