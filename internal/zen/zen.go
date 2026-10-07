// Package zen doles out small doses of wisdom for long coding sessions.
package zen

import "math/rand/v2"

// Aphorisms holds the wisdom that Pick hands out.
var Aphorisms = []string{
	"Small tools, composed well, beat large ones composed poorly.",
	"A session saved is a session earned.",
	"Name it well and the comment writes itself.",
	"Let the agent run; you keep the map.",
	"First make it work, then make it pleasant, then make it fast.",
	"Every daemon deserves a graceful shutdown.",
	"A context cancelled is a lesson learned.",
}

// Pick returns a random aphorism from the collection.
func Pick() string {
	return Aphorisms[rand.IntN(len(Aphorisms))]
}
