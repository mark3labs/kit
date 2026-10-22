package kit_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Run explicitly with KIT_DECISION_LIVE=1. Each case makes a paid live
// request and is skipped when its provider key is not set. The Cloudflare
// case also needs CLOUDFLARE_ACCOUNT_ID (or CF_ACCOUNT_ID).
func TestDecideLive(t *testing.T) {
	if os.Getenv("KIT_DECISION_LIVE") != "1" {
		t.Skip("set KIT_DECISION_LIVE=1 to test decision models live")
	}

	cases := []struct {
		model  string
		env    []string
		images bool
	}{
		{"typesafe/jev-latest", []string{"TYPESAFE_API_KEY", "TYPESAFE_AI_API_KEY"}, true},
		{"openrouter/typesafe/jev-1.14", []string{"OPENROUTER_API_KEY"}, true},
		{"openrouter/cloudflare/clef-flash", []string{"OPENROUTER_API_KEY"}, true},
		{"opencode/jev-1.13", []string{"OPENCODE_API_KEY"}, false},
		{"cloudflare-workers-ai/@cf/cloudflare/clef-flash", []string{"CLOUDFLARE_API_KEY", "CLOUDFLARE_API_TOKEN", "CF_API_TOKEN"}, true},
		{"vercel/typesafe-ai/jev", []string{"AI_GATEWAY_API_KEY", "VERCEL_API_KEY"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			if !anyEnv(tc.env...) {
				t.Skipf("none of %v is set", tc.env)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			res, err := kit.Decide(ctx, kit.DecisionRequest{
				Model: tc.model,
				State: kit.DecisionState{Text: "My card was charged twice for one order and I want my money back today."},
				Questions: []kit.DecisionQuestion{
					kit.BooleanQuestion("refund", "Does the customer ask for a refund?"),
					kit.ChoiceQuestion("team", "Which team should handle this ticket?",
						kit.DecisionOption{Name: "billing", Description: "Payments, charges and refunds"},
						kit.DecisionOption{Name: "shipping", Description: "Delivery problems"},
						kit.DecisionOption{Name: "technical"},
					),
					kit.ScoreQuestion("urgency", "How urgent is this ticket?", "low", "medium", "high"),
				},
			})
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			refund, err := res.Boolean("refund")
			if err != nil || refund.Probability < 0.5 {
				t.Errorf("refund = %+v, %v; want P(true) >= 0.5", refund, err)
			}
			team, err := res.Choice("team")
			if err != nil || team.Choice != "billing" {
				t.Errorf("team = %+v, %v; want billing", team, err)
			}
			urgency, err := res.Score("urgency")
			if err != nil {
				t.Errorf("urgency: %v", err)
			}
			t.Logf("model=%s usage=%+v refund=%.2f team=%s %v urgency=%.2f (level %d)",
				res.Model, res.Usage, refund.Probability, team.Choice, team.Distribution, urgency.Score, urgency.Level)

			if !tc.images {
				return
			}
			res, err = kit.Decide(ctx, kit.DecisionRequest{
				Model: tc.model,
				State: kit.DecisionState{
					Text:   "A photo of a product sample.",
					Images: []kit.DecisionImage{{MediaType: "image/png", Data: solidPNG(t, color.RGBA{R: 220, A: 255})}},
				},
				Questions: []kit.DecisionQuestion{
					kit.ChoiceQuestion("color", "What is the main color of the image?",
						kit.DecisionOption{Name: "red"}, kit.DecisionOption{Name: "green"}, kit.DecisionOption{Name: "blue"},
					),
				},
			})
			if err != nil {
				t.Fatalf("Decide with image: %v", err)
			}
			c, err := res.Choice("color")
			if err != nil || c.Choice != "red" {
				t.Errorf("color = %+v, %v; want red", c, err)
			}
			t.Logf("image: color=%s %v", c.Choice, c.Distribution)
		})
	}
}

func anyEnv(names ...string) bool {
	for _, n := range names {
		if os.Getenv(n) != "" {
			return true
		}
	}
	return false
}

// solidPNG returns a 512x512 PNG of one color. Jev rejects images smaller
// than about 300 pixels per side.
func solidPNG(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for y := range 512 {
		for x := range 512 {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
