package test

import (
	"strings"
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

const decideExtSrc = `package main

import (
	"errors"
	"fmt"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("model=" + ctx.GetDecisionModel())

		res, err := ctx.Decide(ext.DecisionRequest{
			State: ext.DecisionState{Text: "rm -rf /"},
			Questions: []ext.DecisionQuestion{
				ext.BooleanQuestion("destructive", "Does this delete data?"),
				ext.ChoiceQuestion("route", "Which team?",
					ext.DecisionOption{Name: "ops"}, ext.DecisionOption{Name: "dev"}),
				ext.ScoreQuestion("risk", "How risky?", "low", "mid", "high"),
			},
		})
		if errors.Is(err, ext.ErrNoDecisionModel) {
			ctx.Print("no-model")
			return
		}
		if err != nil {
			ctx.Print("error: " + err.Error())
			return
		}

		b, err := res.Boolean("destructive")
		if err != nil {
			ctx.Print("error: " + err.Error())
			return
		}
		c, _ := res.Choice("route")
		p, ok := c.Prob("ops")
		s, _ := res.Score("risk")
		_, wrong := res.Score("destructive")
		ctx.Print(fmt.Sprintf("p=%.2f choice=%s ops=%.2f/%v score=%.1f level=%d wrongType=%v",
			b.Probability, c.Choice, p, ok, s.Score, s.Level,
			errors.Is(wrong, ext.ErrWrongDecisionAnswerType)))
	})
}
`

// TestDecideCrossesYaegiBoundary checks that the decision types, helpers,
// result accessors and sentinel errors work from interpreted code.
func TestDecideCrossesYaegiBoundary(t *testing.T) {
	h := New(t)
	h.LoadString(decideExtSrc, "decide.go")

	mock := h.Context()
	mock.DecisionModel = "openrouter/typesafe/jev-1.14"
	mock.DecideResult = extensions.DecisionResult{Answers: []extensions.DecisionAnswer{
		{ID: "destructive", Type: extensions.DecisionBoolean, Probability: 0.93},
		{ID: "route", Type: extensions.DecisionChoice, Choice: "ops", Distribution: []extensions.DecisionProbability{
			{Index: 0, Name: "ops", Probability: 0.7}, {Index: 1, Name: "dev", Probability: 0.3},
		}},
		{ID: "risk", Type: extensions.DecisionScore, Score: 1.8, Level: 2},
	}}
	h.Runner().SetContext(mock.ToContext())

	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	prints := mock.GetPrints()
	want := []string{
		"model=openrouter/typesafe/jev-1.14",
		"p=0.93 choice=ops ops=0.70/true score=1.8 level=2 wrongType=true",
	}
	if strings.Join(prints, "\n") != strings.Join(want, "\n") {
		t.Fatalf("prints = %q, want %q", prints, want)
	}

	if len(mock.DecideRequests) != 1 {
		t.Fatalf("got %d Decide requests, want 1", len(mock.DecideRequests))
	}
	req := mock.DecideRequests[0]
	if req.Model != "openrouter/typesafe/jev-1.14" {
		t.Errorf("Model = %q, want the configured default", req.Model)
	}
	if len(req.Questions) != 3 || req.Questions[1].Options[1].Name != "dev" || req.Questions[2].Levels[2] != "high" {
		t.Errorf("questions did not cross intact: %+v", req.Questions)
	}
}

// TestDecideNoModelFromYaegi checks that an extension can detect a missing
// decision-model with errors.Is.
func TestDecideNoModelFromYaegi(t *testing.T) {
	h := New(t)
	h.LoadString(decideExtSrc, "decide.go")

	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	prints := h.Context().GetPrints()
	want := []string{"model=", "no-model"}
	if strings.Join(prints, "\n") != strings.Join(want, "\n") {
		t.Fatalf("prints = %q, want %q", prints, want)
	}
}

// TestDecisionModelsFromMock checks that the mock wires GetDecisionModels,
// both for a direct call and from interpreted code.
func TestDecisionModelsFromMock(t *testing.T) {
	h := New(t)
	h.LoadString(`package main

import (
	"fmt"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		for _, m := range ctx.GetDecisionModels() {
			ctx.Print(fmt.Sprintf("%s wire=%s images=%v", m.Model, m.Wire, m.AcceptsImages))
		}
	})
}
`, "models.go")

	mock := h.Context()
	if got := mock.ToContext().GetDecisionModels(); got != nil {
		t.Fatalf("empty mock: GetDecisionModels = %v, want nil", got)
	}
	mock.DecisionModels = []extensions.DecisionModelInfo{
		{Model: "openrouter/typesafe/jev-1.14", Wire: "systemone", AcceptsImages: true},
	}
	if got := mock.ToContext().GetDecisionModels(); len(got) != 1 || got[0].Wire != "systemone" {
		t.Fatalf("GetDecisionModels = %v", got)
	}

	if _, err := h.Emit(extensions.SessionStartEvent{SessionID: "test"}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	want := "openrouter/typesafe/jev-1.14 wire=systemone images=true"
	if prints := mock.GetPrints(); len(prints) != 1 || prints[0] != want {
		t.Fatalf("prints = %q, want [%q]", prints, want)
	}
}
