package kit

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
)

// fixedDecisionModel answers with a fixed result and records the request.
type fixedDecisionModel struct {
	res   *DecisionResult
	err   error
	calls atomic.Int32
	last  atomic.Value // DecisionRequest
}

func (m *fixedDecisionModel) Decide(_ context.Context, req DecisionRequest) (*DecisionResult, error) {
	m.calls.Add(1)
	m.last.Store(req)
	return m.res, m.err
}

func (m *fixedDecisionModel) factory(gotModel *atomic.Value) DecisionProviderFactory {
	return func(_ context.Context, cfg *ProviderConfig, model string) (DecisionModel, error) {
		if gotModel != nil {
			gotModel.Store(model)
		}
		return m, nil
	}
}

func triageQuestions() []DecisionQuestion {
	return []DecisionQuestion{
		BooleanQuestion("refund", "Does the customer request a refund?"),
		ChoiceQuestion("team", "Which team?", DecisionOption{Name: "billing"}, DecisionOption{Name: "technical"}),
		ScoreQuestion("urgency", "How urgent?", "low", "medium", "high"),
	}
}

func TestDecisionProvider_InstanceFactory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := &fixedDecisionModel{res: &DecisionResult{
		Model: "my-decider-1",
		// Out of question order, with a score distribution by level name.
		Answers: []DecisionAnswer{
			{ID: "urgency", Type: DecisionScore, Score: 1.6, Distribution: []DecisionProbability{
				{Name: "high", Probability: 0.7}, {Name: "low", Probability: 0.1}, {Name: "medium", Probability: 0.2},
			}},
			{ID: "team", Type: DecisionChoice, Choice: "billing"},
			{ID: "refund", Type: DecisionBoolean, Probability: 0.9},
		},
	}}
	var gotModel atomic.Value
	k := newProviderTestKit(t, &Options{
		Model:             "custom/test",
		ProviderURL:       "http://unused.test/v1",
		ProviderAPIKey:    "fake",
		Bare:              true,
		DecisionModel:     "Local/my-decider",
		DecisionProviders: map[string]DecisionProviderFactory{"local": m.factory(&gotModel)},
	})

	res, err := k.Decide(context.Background(), DecisionRequest{
		State:     DecisionState{Text: "My card was charged twice."},
		Questions: triageQuestions(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := gotModel.Load(); got != "my-decider" {
		t.Errorf("factory model = %v, want my-decider", got)
	}
	if req := m.last.Load().(DecisionRequest); req.Model != "Local/my-decider" {
		t.Errorf("request model = %q", req.Model)
	}
	ids := []string{res.Answers[0].ID, res.Answers[1].ID, res.Answers[2].ID}
	if !slices.Equal(ids, []string{"refund", "team", "urgency"}) {
		t.Errorf("answer order = %v", ids)
	}
	if a, err := res.Boolean("refund"); err != nil || a.Probability != 0.9 {
		t.Errorf("refund = %+v, %v", a, err)
	}
	if a, err := res.Choice("team"); err != nil || a.Choice != "billing" {
		t.Errorf("team = %+v, %v", a, err)
	}
	a, err := res.Score("urgency")
	if err != nil || a.Level != 2 || len(a.Distribution) != 3 || a.Distribution[0].Name != "low" || a.Distribution[2].Index != 2 {
		t.Errorf("urgency = %+v, %v", a, err)
	}
	if res.Model != "my-decider-1" {
		t.Errorf("result model = %q", res.Model)
	}
}

func TestDecisionProvider_ProcessWide(t *testing.T) {
	m := &fixedDecisionModel{res: &DecisionResult{Answers: []DecisionAnswer{
		{ID: "ok", Type: DecisionBoolean, Probability: 0.25},
	}}}
	if err := RegisterDecisionProvider("testdecide", m.factory(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { UnregisterDecisionProvider("testdecide") })
	if !slices.Contains(RegisteredDecisionProviders(), "testdecide") {
		t.Errorf("RegisteredDecisionProviders = %v", RegisteredDecisionProviders())
	}

	res, err := Decide(context.Background(), DecisionRequest{
		Model:     "TestDecide/any",
		State:     DecisionState{Text: "x"},
		Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := res.Boolean("ok"); a.Probability != 0.25 {
		t.Errorf("answer = %+v", a)
	}

	if !UnregisterDecisionProvider("testdecide") {
		t.Error("UnregisterDecisionProvider = false")
	}
	if _, err := Decide(context.Background(), DecisionRequest{
		Model:     "testdecide/any",
		State:     DecisionState{Text: "x"},
		Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")},
	}); !errors.Is(err, ErrNotDecisionModel) {
		t.Errorf("after unregister: error = %v, want ErrNotDecisionModel", err)
	}
}

func TestDecisionProvider_ValidatesRequestBeforeCall(t *testing.T) {
	m := &fixedDecisionModel{}
	if err := RegisterDecisionProvider("testvalidate", m.factory(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { UnregisterDecisionProvider("testvalidate") })

	for name, req := range map[string]DecisionRequest{
		"empty state":   {Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")}},
		"no questions":  {State: DecisionState{Text: "x"}},
		"one option":    {State: DecisionState{Text: "x"}, Questions: []DecisionQuestion{ChoiceQuestion("c", "c?", DecisionOption{Name: "a"})}},
		"untyped image": {State: DecisionState{Images: []DecisionImage{{Data: []byte{1}}}}, Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")}},
	} {
		req.Model = "testvalidate/m"
		if _, err := Decide(context.Background(), req); !errors.Is(err, ErrInvalidDecisionRequest) {
			t.Errorf("%s: error = %v, want ErrInvalidDecisionRequest", name, err)
		}
	}
	if n := m.calls.Load(); n != 0 {
		t.Errorf("model calls = %d, want 0", n)
	}

	// No wire limit on the question count, and any image type.
	many := make([]DecisionQuestion, 100)
	answers := make([]DecisionAnswer, 100)
	for i := range many {
		id := string(rune('a'+i%26)) + string(rune('a'+i/26))
		many[i] = BooleanQuestion(id, "ok?")
		answers[i] = DecisionAnswer{ID: id, Type: DecisionBoolean, Probability: 0.5}
	}
	m.res = &DecisionResult{Answers: answers}
	if _, err := Decide(context.Background(), DecisionRequest{
		Model:     "testvalidate/m",
		State:     DecisionState{Images: []DecisionImage{{MediaType: "image/gif", Data: []byte{1}}}},
		Questions: many,
	}); err != nil {
		t.Errorf("100 questions with a gif: %v", err)
	}
}

func TestDecisionProvider_ChecksResult(t *testing.T) {
	q := []DecisionQuestion{
		BooleanQuestion("ok", "ok?"),
		ChoiceQuestion("team", "team?", DecisionOption{Name: "a"}, DecisionOption{Name: "b"}),
	}
	good := DecisionAnswer{ID: "team", Type: DecisionChoice, Choice: "a"}
	for name, answers := range map[string][]DecisionAnswer{
		"missing answer": {good},
		"wrong type":     {{ID: "ok", Type: DecisionScore}, good},
		"bad prob":       {{ID: "ok", Type: DecisionBoolean, Probability: 1.5}, good},
		"unknown choice": {{ID: "ok", Type: DecisionBoolean}, {ID: "team", Type: DecisionChoice, Choice: "c"}},
		"extra answer":   {{ID: "ok", Type: DecisionBoolean}, good, {ID: "x", Type: DecisionBoolean}},
		"duplicate":      {{ID: "ok", Type: DecisionBoolean}, good, good},
	} {
		m := &fixedDecisionModel{res: &DecisionResult{Answers: answers}}
		_, err := decide(context.Background(), nil, map[string]DecisionProviderFactory{"local": m.factory(nil)}, DecisionRequest{
			Model: "local/m", State: DecisionState{Text: "x"}, Questions: q,
		})
		if !errors.Is(err, ErrInvalidDecisionResponse) {
			t.Errorf("%s: error = %v, want ErrInvalidDecisionResponse", name, err)
		}
	}

	// A refused answer passes and keeps only its ID and type.
	m := &fixedDecisionModel{res: &DecisionResult{Answers: []DecisionAnswer{
		{ID: "ok", Type: DecisionBoolean, Refused: true, Probability: 7},
		good,
	}}}
	res, err := decide(context.Background(), nil, map[string]DecisionProviderFactory{"local": m.factory(nil)}, DecisionRequest{
		Model: "local/m", State: DecisionState{Text: "x"}, Questions: q,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := res.Boolean("ok"); !errors.Is(err, ErrDecisionRefused) {
		t.Errorf("refused: error = %v", err)
	}
	// The model's result is not changed.
	if m.res.Answers[0].Probability != 7 {
		t.Error("model result was changed")
	}
}

func TestDecisionProvider_FactoryErrors(t *testing.T) {
	req := DecisionRequest{Model: "local/m", State: DecisionState{Text: "x"}, Questions: []DecisionQuestion{BooleanQuestion("ok", "ok?")}}
	boom := errors.New("backend not loaded")

	failing := func(context.Context, *ProviderConfig, string) (DecisionModel, error) { return nil, boom }
	if _, err := decide(context.Background(), nil, map[string]DecisionProviderFactory{"local": failing}, req); !errors.Is(err, boom) {
		t.Errorf("factory error = %v", err)
	}
	empty := func(context.Context, *ProviderConfig, string) (DecisionModel, error) { return nil, nil }
	if _, err := decide(context.Background(), nil, map[string]DecisionProviderFactory{"local": empty}, req); err == nil {
		t.Error("nil model: no error")
	}
	m := DecisionModelFunc(func(context.Context, DecisionRequest) (*DecisionResult, error) { return nil, boom })
	fromFunc := func(context.Context, *ProviderConfig, string) (DecisionModel, error) { return m, nil }
	if _, err := decide(context.Background(), nil, map[string]DecisionProviderFactory{"local": fromFunc}, req); !errors.Is(err, boom) {
		t.Errorf("model error = %v", err)
	}
}

func TestDecisionProvider_InvalidOptions(t *testing.T) {
	f := (&fixedDecisionModel{}).factory(nil)
	for name, m := range map[string]map[string]DecisionProviderFactory{
		"empty name": {"": f},
		"slash":      {"a/b": f},
		"nil":        {"local": nil},
		"case dup":   {"Local": f, "local": f},
	} {
		if _, err := normalizeDecisionProviders(m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := RegisterDecisionProvider("a/b", f); err == nil {
		t.Error("RegisterDecisionProvider with a slash: no error")
	}
}
