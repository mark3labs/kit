package decision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/kit/internal/models"
)

var testQuestions = []Question{
	{ID: "refund", Type: Boolean, Instructions: "Is a refund requested?"},
	{ID: "team", Type: Choice, Instructions: "Which team?", Options: []Option{
		{Name: "technical"}, {Name: "billing", Description: "Payments"}, {Name: "sales"},
	}},
	{ID: "urgency", Type: Score, Instructions: "How urgent?", Levels: []string{"low", "medium", "high"}},
}

const systemOneAnswers = `{
  "model": "jev-1.14.0",
  "answers": {
    "refund": {"type": "noul", "noul": 0.97},
    "team": {"type": "choice", "choice": "billing", "probabilities": {"technical": 0.05, "billing": 0.9, "sales": 0.05}, "confidence": 0.88},
    "urgency": {"type": "score", "score": 1.2, "legend": {"0": "low", "1": "medium", "2": "high"}, "probabilities": {"0": 0.1, "1": 0.6, "2": 0.3}, "confidence": 0.4}
  },
  "usage": {"input_tokens": 300, "output_tokens": 20}
}`

// capture records the last request a test server received.
type capture struct {
	header http.Header
	body   []byte
}

func server(t *testing.T, status int, response string, c *capture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if c != nil {
			c.header, c.body = r.Header.Clone(), body
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func endpoint(url string, wire models.DecisionWire, images models.DecisionImageMode) *models.DecisionEndpoint {
	return &models.DecisionEndpoint{
		Provider: "test", Model: "test-model", Wire: wire, URL: url, APIKey: "key",
		Images: images, ImageTypes: []string{"image/png", "image/jpeg"}, AcceptsImages: true,
	}
}

func checkAnswers(t *testing.T, res *Result) {
	t.Helper()
	refund, err := res.Boolean("refund")
	if err != nil || refund.Probability != 0.97 {
		t.Errorf("refund = %+v, %v", refund, err)
	}
	team, err := res.Choice("team")
	if err != nil || team.Choice != "billing" {
		t.Fatalf("team = %+v, %v", team, err)
	}
	// The distribution follows question order, not wire order.
	if len(team.Distribution) != 3 || team.Distribution[0].Name != "technical" || team.Distribution[1].Probability != 0.9 {
		t.Errorf("team distribution = %+v", team.Distribution)
	}
	if p, ok := team.Prob("billing"); !ok || p != 0.9 {
		t.Errorf("Prob(billing) = %v, %v", p, ok)
	}
	urgency, err := res.Score("urgency")
	if err != nil || urgency.Score != 1.2 || urgency.Level != 1 || urgency.Distribution[2].Name != "high" {
		t.Errorf("urgency = %+v, %v", urgency, err)
	}
	if _, err := res.Choice("refund"); !errors.Is(err, ErrWrongAnswerType) {
		t.Errorf("Choice(refund) error = %v, want ErrWrongAnswerType", err)
	}
	if _, err := res.Boolean("missing"); !errors.Is(err, ErrNoAnswer) {
		t.Errorf("Boolean(missing) error = %v, want ErrNoAnswer", err)
	}
}

func TestSystemOneJSON(t *testing.T) {
	var c capture
	srv := server(t, 200, systemOneAnswers, &c)
	ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)

	res, err := Decide(context.Background(), ep, Request{State: State{Text: "charged twice"}, Questions: testQuestions})
	if err != nil {
		t.Fatal(err)
	}
	checkAnswers(t, res)
	if res.Model != "jev-1.14.0" || res.Usage.InputTokens != 300 {
		t.Errorf("model=%s usage=%+v", res.Model, res.Usage)
	}

	if got := c.header.Get("Authorization"); got != "Bearer key" {
		t.Errorf("Authorization = %q", got)
	}
	want := `{"model":"test-model","state":"charged twice","questions":{` +
		`"refund":{"type":"noul","instructions":"Is a refund requested?"},` +
		`"team":{"type":"choice","instructions":"Which team?","criteria":{"technical":null,"billing":"Payments","sales":null}},` +
		`"urgency":{"type":"score","instructions":"How urgent?","criteria":["low","medium","high"]}}}`
	if string(c.body) != want {
		t.Errorf("body:\n got %s\nwant %s", c.body, want)
	}
}

func TestSystemOneJSONState(t *testing.T) {
	var c capture
	srv := server(t, 200, systemOneAnswers, &c)
	ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)
	_, err := Decide(context.Background(), ep, Request{State: State{JSON: map[string]any{"ticket": "x"}}, Questions: testQuestions})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c.body), `"state":{"ticket":"x"}`) {
		t.Errorf("JSON state is not structured: %s", c.body)
	}
}

func TestSystemOneMultipartImages(t *testing.T) {
	var c capture
	srv := server(t, 200, systemOneAnswers, &c)
	ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)
	png := []byte("\x89PNG fake")

	_, err := Decide(context.Background(), ep, Request{
		State:     State{Text: "a photo", Images: []Image{{MediaType: "image/png", Data: png}}},
		Questions: testQuestions,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.header.Get("TypeSafe-Danger-Zone-Images") != "true" {
		t.Error("missing TypeSafe-Danger-Zone-Images header")
	}
	_, params, err := mime.ParseMediaType(c.header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	r := multipart.NewReader(strings.NewReader(string(c.body)), params["boundary"])

	part, err := r.NextPart()
	if err != nil || part.FormName() != "request" {
		t.Fatalf("first part = %v, %v; want request", part, err)
	}
	var req map[string]any
	if err := json.NewDecoder(part).Decode(&req); err != nil {
		t.Fatal(err)
	}
	if req["state"] != "a photo\nkit_image_1" {
		t.Errorf("state = %q; want the image reference", req["state"])
	}
	asset := req["assets"].(map[string]any)["kit_image_1"].(map[string]any)
	if asset["type"] != "image" || asset["part"] != "image-0" {
		t.Errorf("asset = %v", asset)
	}

	part, err = r.NextPart()
	if err != nil || part.FormName() != "image-0" || part.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("image part = %v, %v", part, err)
	}
	if data, _ := io.ReadAll(part); string(data) != string(png) {
		t.Errorf("image data = %q", data)
	}
}

func TestSystemOneInlineImages(t *testing.T) {
	png := []byte("\x89PNG fake")
	b64 := base64.StdEncoding.EncodeToString(png)
	tests := []struct {
		mode models.DecisionImageMode
		want string
	}{
		{models.DecisionImagesDataURL, `"images":["data:image/png;base64,` + b64 + `"]`},
		{models.DecisionImagesBase64, `"images":["` + b64 + `"]`},
		{models.DecisionImagesImageURLParts, `"state":[{"type":"text","text":"a photo"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + b64 + `"}}]`},
	}
	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			var c capture
			srv := server(t, 200, systemOneAnswers, &c)
			ep := endpoint(srv.URL, models.DecisionWireSystemOne, tc.mode)
			_, err := Decide(context.Background(), ep, Request{
				State:     State{Text: "a photo", Images: []Image{{MediaType: "image/png", Data: png}}},
				Questions: testQuestions,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(c.body), tc.want) {
				t.Errorf("body %s\ndoes not contain %s", c.body, tc.want)
			}
			if c.header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q", c.header.Get("Content-Type"))
			}
		})
	}
}

func TestWorkersAIEnvelope(t *testing.T) {
	srv := server(t, 200, `{"result": `+systemOneAnswers+`, "success": true, "errors": [], "messages": []}`, nil)
	ep := endpoint(srv.URL, models.DecisionWireWorkersAI, models.DecisionImagesDataURL)
	res, err := Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: testQuestions})
	if err != nil {
		t.Fatal(err)
	}
	checkAnswers(t, res)

	srv = server(t, 200, `{"result": null, "success": false, "errors": [{"code": 5006, "message": "bad model"}]}`, nil)
	ep = endpoint(srv.URL, models.DecisionWireWorkersAI, models.DecisionImagesDataURL)
	_, err = Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: testQuestions})
	if !errors.Is(err, ErrInvalidResponse) || !strings.Contains(err.Error(), "bad model") {
		t.Errorf("error = %v, want the envelope error", err)
	}
}

func TestGateway(t *testing.T) {
	var c capture
	srv := server(t, 200, `{
	  "model": "typesafe-ai/jev",
	  "answers": {
	    "refund": {"type": "boolean", "probability": 0.97},
	    "team": {"type": "choice", "choice": "billing", "probabilities": {"technical": 0.05, "billing": 0.9, "sales": 0.05}},
	    "urgency": {"type": "score", "score": 1.2, "probabilities": {"0": 0.1, "1": 0.6, "2": 0.3}}
	  },
	  "usage": {"inputTokens": 280, "outputTokens": 21},
	  "providerMetadata": {"typesafe": {"confidence": {"team": 0.88}}, "gateway": {"cost": "0.0001"}}
	}`, &c)
	ep := endpoint(srv.URL, models.DecisionWireGateway, models.DecisionImagesStateParts)

	res, err := Decide(context.Background(), ep, Request{
		State:           State{Text: "charged twice", Images: []Image{{MediaType: "image/jpeg", Data: []byte("jpg")}}},
		Questions:       testQuestions,
		ProviderOptions: map[string]any{"gateway": map[string]any{"zeroDataRetention": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	checkAnswers(t, res)
	if res.Usage.InputTokens != 280 {
		t.Errorf("usage = %+v", res.Usage)
	}
	if team, _ := res.Choice("team"); team.Confidence == nil || *team.Confidence != 0.88 {
		t.Errorf("team confidence = %v", team.Confidence)
	}

	for k, v := range map[string]string{
		"ai-model-id": "test-model", "ai-decision-model-specification-version": "4",
		"ai-gateway-protocol-version": "0.0.1", "ai-gateway-auth-method": "api-key",
	} {
		if got := c.header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	body := string(c.body)
	for _, want := range []string{
		`"stateParts":[{"type":"text","text":"charged twice"},{"type":"file","mediaType":"image/jpeg","data":{"type":"data","data":"` + base64.StdEncoding.EncodeToString([]byte("jpg")) + `"}}]`,
		`"refund":{"type":"boolean"`,
		`"criteria":{"technical":"technical","billing":"Payments","sales":"sales"}`,
		`"providerOptions":{"gateway":{"zeroDataRetention":true}}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s\ndoes not contain %s", body, want)
		}
	}
}

func TestRefusal(t *testing.T) {
	srv := server(t, 200, `{"answers": {"refund": {"type": "refusal"}}}`, nil)
	ep := endpoint(srv.URL, models.DecisionWireGateway, models.DecisionImagesStateParts)
	res, err := Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: testQuestions[:1]})
	if err != nil {
		t.Fatal(err)
	}
	a, err := res.Boolean("refund")
	if !errors.Is(err, ErrRefused) || !a.Refused {
		t.Errorf("answer = %+v, %v; want a refusal", a, err)
	}
}

func TestInvalidResponses(t *testing.T) {
	tests := map[string]string{
		"missing answer": `{"answers": {}}`,
		"wrong type":     `{"answers": {"refund": {"type": "choice", "choice": "x"}}}`,
		"unknown choice": `{"answers": {"team": {"type": "choice", "choice": "legal"}}}`,
		"score range":    `{"answers": {"urgency": {"type": "score", "score": 7}}}`,
		"not json":       `<html>`,
	}
	questions := map[string][]Question{
		"unknown choice": {testQuestions[1]},
		"score range":    {testQuestions[2]},
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			qs := questions[name]
			if qs == nil {
				qs = testQuestions[:1]
			}
			srv := server(t, 200, body, nil)
			ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)
			_, err := Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: qs})
			if !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("error = %v, want ErrInvalidResponse", err)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	ep := endpoint("http://unused.test", models.DecisionWireSystemOne, models.DecisionImagesMultipart)
	noImages := *ep
	noImages.AcceptsImages = false
	limited := *ep
	limited.MaxImages = 1
	img := Image{MediaType: "image/png", Data: []byte("x")}

	tests := []struct {
		name string
		ep   *models.DecisionEndpoint
		req  Request
	}{
		{"empty state", ep, Request{Questions: testQuestions}},
		{"no questions", ep, Request{State: State{Text: "x"}}},
		{"duplicate id", ep, Request{State: State{Text: "x"}, Questions: []Question{testQuestions[0], testQuestions[0]}}},
		{"one option", ep, Request{State: State{Text: "x"}, Questions: []Question{{ID: "a", Type: Choice, Options: []Option{{Name: "x"}}}}}},
		{"duplicate option", ep, Request{State: State{Text: "x"}, Questions: []Question{{ID: "a", Type: Choice, Options: []Option{{Name: "x"}, {Name: "x"}}}}}},
		{"one level", ep, Request{State: State{Text: "x"}, Questions: []Question{{ID: "a", Type: Score, Levels: []string{"x"}}}}},
		{"boolean without text", ep, Request{State: State{Text: "x"}, Questions: []Question{{ID: "a", Type: Boolean}}}},
		{"unknown type", ep, Request{State: State{Text: "x"}, Questions: []Question{{ID: "a", Type: "box"}}}},
		{"images not accepted", &noImages, Request{State: State{Images: []Image{img}}, Questions: testQuestions}},
		{"too many images", &limited, Request{State: State{Images: []Image{img, img}}, Questions: testQuestions}},
		{"media type", ep, Request{State: State{Images: []Image{{MediaType: "image/gif", Data: []byte("x")}}}, Questions: testQuestions}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decide(context.Background(), tc.ep, tc.req)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"message": "slow down"}`)
			return
		}
		_, _ = io.WriteString(w, systemOneAnswers)
	}))
	t.Cleanup(srv.Close)
	ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)

	res, err := Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: testQuestions})
	if err != nil {
		t.Fatal(err)
	}
	checkAnswers(t, res)
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestAPIErrorNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"detail": {"error_type": "api_usage_error", "message": "Invalid image dimensions."}}`)
	}))
	t.Cleanup(srv.Close)
	ep := endpoint(srv.URL, models.DecisionWireSystemOne, models.DecisionImagesMultipart)

	_, err := Decide(context.Background(), ep, Request{State: State{Text: "x"}, Questions: testQuestions})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || !strings.Contains(apiErr.Message, "Invalid image dimensions") {
		t.Fatalf("error = %v, want APIError 400 with the provider message", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}
