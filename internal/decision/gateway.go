package decision

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/kit/internal/models"
)

// Protocol headers of the Vercel AI Gateway decision model endpoint.
const (
	gatewayProtocolVersion = "0.0.1"
	gatewayDecisionSpec    = "4"
)

// gatewayStateParts converts the state to the gateway's ordered state parts:
// text, then JSON, then one file part per image.
func gatewayStateParts(s State) []any {
	var parts []any
	if s.Text != "" {
		parts = append(parts, orderedObject{{"type", "text"}, {"text", s.Text}})
	}
	if s.JSON != nil {
		parts = append(parts, orderedObject{{"type", "json"}, {"value", s.JSON}})
	}
	for _, img := range s.Images {
		part := orderedObject{
			{"type", "file"},
			{"mediaType", img.MediaType},
			{"data", orderedObject{{"type", "data"}, {"data", base64.StdEncoding.EncodeToString(img.Data)}}},
		}
		if img.Filename != "" {
			part = append(part, orderedField{"filename", img.Filename})
		}
		parts = append(parts, part)
	}
	return parts
}

func encodeGateway(ep *models.DecisionEndpoint, req Request) ([]byte, string, map[string]string, error) {
	body := orderedObject{
		{"stateParts", gatewayStateParts(req.State)},
		{"questions", wireQuestions(req.Questions, string(Boolean), false)},
	}
	if len(req.ProviderOptions) > 0 {
		body = append(body, orderedField{"providerOptions", req.ProviderOptions})
	}
	js, err := json.Marshal(body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("encode decision request: %w", err)
	}
	headers := map[string]string{
		"ai-gateway-protocol-version":             gatewayProtocolVersion,
		"ai-gateway-auth-method":                  "api-key",
		"ai-decision-model-specification-version": gatewayDecisionSpec,
		"ai-model-id":                             ep.Model,
	}
	return js, "application/json", headers, nil
}

type gatewayResponse struct {
	Model   string                   `json:"model"`
	Answers map[string]gatewayAnswer `json:"answers"`
	Usage   *struct {
		InputTokens  int64 `json:"inputTokens"`
		OutputTokens int64 `json:"outputTokens"`
	} `json:"usage"`
	ProviderMetadata map[string]any `json:"providerMetadata"`
}

type gatewayAnswer struct {
	Type          string             `json:"type"`
	Probability   *float64           `json:"probability"`
	Choice        string             `json:"choice"`
	Score         *float64           `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func decodeGateway(req Request, data []byte) (*Result, error) {
	var resp gatewayResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	res := &Result{Model: resp.Model, Metadata: resp.ProviderMetadata}
	if resp.Usage != nil {
		res.Usage = Usage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
	}
	confidence := gatewayConfidence(resp.ProviderMetadata)
	for _, q := range req.Questions {
		wa, ok := resp.Answers[q.ID]
		if !ok {
			return nil, fmt.Errorf("%w: no answer for question %q", ErrInvalidResponse, q.ID)
		}
		var a Answer
		var err error
		switch {
		case wa.Type == "refusal":
			a = Answer{ID: q.ID, Type: q.Type, Refused: true}
		case q.Type == Boolean && wa.Type == "boolean" && wa.Probability != nil:
			a, err = buildAnswer(q, "", 0, *wa.Probability, nil, nil)
		case q.Type == Choice && wa.Type == "choice":
			a, err = buildAnswer(q, wa.Choice, 0, 0, wa.Probabilities, confidence[q.ID])
		case q.Type == Score && wa.Type == "score" && wa.Score != nil:
			a, err = buildAnswer(q, "", *wa.Score, 0, wa.Probabilities, confidence[q.ID])
		default:
			return nil, fmt.Errorf("%w: question %q is %s, answer is %q", ErrInvalidResponse, q.ID, q.Type, wa.Type)
		}
		if err != nil {
			return nil, err
		}
		res.Answers = append(res.Answers, a)
	}
	return res, nil
}

// gatewayConfidence reads per-question confidence from provider metadata.
// Providers put it under their own namespace, for example
// {"typesafe": {"confidence": {"route": 0.91}}}.
func gatewayConfidence(meta map[string]any) map[string]*float64 {
	out := map[string]*float64{}
	for _, ns := range meta {
		nsMap, ok := ns.(map[string]any)
		if !ok {
			continue
		}
		conf, ok := nsMap["confidence"].(map[string]any)
		if !ok {
			continue
		}
		for id, v := range conf {
			if f, ok := v.(float64); ok {
				out[id] = &f
			}
		}
	}
	return out
}
