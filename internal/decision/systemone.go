package decision

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/mark3labs/kit/internal/models"
)

// orderedObject is a JSON object that keeps its key order. encoding/json
// sorts map keys, which would reorder choice options and questions.
type orderedObject []orderedField

type orderedField struct {
	Key   string
	Value any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(f.Key)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(f.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// wireQuestions encodes the questions as an ordered object. booleanType is
// the wire name of boolean questions ("noul" on SystemOne). nullDescription
// sends an empty option description as null (SystemOne); otherwise the
// option name stands in for it.
func wireQuestions(qs []Question, booleanType string, nullDescription bool) orderedObject {
	out := make(orderedObject, 0, len(qs))
	for _, q := range qs {
		wq := orderedObject{{"type", string(q.Type)}}
		if q.Type == Boolean {
			wq[0].Value = booleanType
		}
		if q.Instructions != "" {
			wq = append(wq, orderedField{"instructions", q.Instructions})
		}
		switch q.Type {
		case Boolean:
			if q.True != "" || q.False != "" {
				criteria := orderedObject{}
				if q.True != "" {
					criteria = append(criteria, orderedField{"true", q.True})
				}
				if q.False != "" {
					criteria = append(criteria, orderedField{"false", q.False})
				}
				wq = append(wq, orderedField{"criteria", criteria})
			}
		case Choice:
			criteria := make(orderedObject, 0, len(q.Options))
			for _, o := range q.Options {
				var desc any // null lets the name describe itself
				switch {
				case o.Description != "":
					desc = o.Description
				case !nullDescription:
					desc = o.Name
				}
				criteria = append(criteria, orderedField{o.Name, desc})
			}
			wq = append(wq, orderedField{"criteria", criteria})
		case Score:
			wq = append(wq, orderedField{"criteria", q.Levels})
		}
		out = append(out, orderedField{q.ID, wq})
	}
	return out
}

// imageRef is the asset name that stands for image i in a multipart state.
func imageRef(i int) string { return fmt.Sprintf("kit_image_%d", i+1) }

// systemOneState returns the "state" value. A JSON-only state stays
// structured; otherwise the parts join into one string, which on the
// multipart wire also carries the image references.
func systemOneState(s State, multipartImages bool) (any, error) {
	if s.JSON != nil && s.Text == "" && (!multipartImages || len(s.Images) == 0) {
		return s.JSON, nil
	}
	var parts []string
	if s.Text != "" {
		parts = append(parts, s.Text)
	}
	if s.JSON != nil {
		b, err := json.Marshal(s.JSON)
		if err != nil {
			return nil, fmt.Errorf("%w: state JSON: %v", ErrInvalidRequest, err)
		}
		parts = append(parts, string(b))
	}
	if multipartImages {
		for i := range s.Images {
			parts = append(parts, imageRef(i))
		}
	}
	return strings.Join(parts, "\n"), nil
}

// imageURLState returns the state as an array of parts: the text state
// (or the JSON state as text), then one image_url part per image.
func imageURLState(s State, state any) []any {
	var parts []any
	text, ok := state.(string)
	if !ok {
		b, _ := json.Marshal(state) // state came from json-encodable input
		text = string(b)
	}
	if text != "" {
		parts = append(parts, orderedObject{{"type", "text"}, {"text", text}})
	}
	for _, img := range s.Images {
		url := "data:" + img.MediaType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
		parts = append(parts, orderedObject{{"type", "image_url"}, {"image_url", orderedObject{{"url", url}}}})
	}
	return parts
}

// encodeSystemOne builds a SystemOne request body (also used, without
// multipart, by Workers AI). It returns the body and its content type.
func encodeSystemOne(ep *models.DecisionEndpoint, req Request) ([]byte, string, map[string]string, error) {
	multipartImages := ep.Images == models.DecisionImagesMultipart && len(req.State.Images) > 0
	state, err := systemOneState(req.State, multipartImages)
	if err != nil {
		return nil, "", nil, err
	}
	if ep.Images == models.DecisionImagesImageURLParts && len(req.State.Images) > 0 {
		state = imageURLState(req.State, state)
	}
	body := orderedObject{
		{"model", ep.Model},
		{"state", state},
	}

	if len(req.State.Images) > 0 {
		switch ep.Images {
		case models.DecisionImagesDataURL, models.DecisionImagesBase64:
			images := make([]string, len(req.State.Images))
			for i, img := range req.State.Images {
				data := base64.StdEncoding.EncodeToString(img.Data)
				if ep.Images == models.DecisionImagesDataURL {
					data = "data:" + img.MediaType + ";base64," + data
				}
				images[i] = data
			}
			body = append(body, orderedField{"images", images})
		case models.DecisionImagesMultipart:
			assets := make(orderedObject, len(req.State.Images))
			for i := range req.State.Images {
				assets[i] = orderedField{imageRef(i), orderedObject{{"type", "image"}, {"part", fmt.Sprintf("image-%d", i)}}}
			}
			body = append(body, orderedField{"assets", assets})
		}
	}
	body = append(body, orderedField{"questions", wireQuestions(req.Questions, "noul", true)})

	js, err := json.Marshal(body)
	if err != nil {
		return nil, "", nil, fmt.Errorf("encode decision request: %w", err)
	}
	if !multipartImages {
		return js, "application/json", nil, nil
	}

	// Multipart: the JSON request first, then one part per image.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := writePart(w, "request", "request.json", "application/json", js); err != nil {
		return nil, "", nil, err
	}
	for i, img := range req.State.Images {
		name := img.Filename
		if name == "" {
			name = fmt.Sprintf("image-%d.%s", i, extensionFor(img.MediaType))
		}
		if err := writePart(w, fmt.Sprintf("image-%d", i), name, img.MediaType, img.Data); err != nil {
			return nil, "", nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", nil, fmt.Errorf("encode decision request: %w", err)
	}
	headers := map[string]string{"TypeSafe-Danger-Zone-Images": "true"}
	return buf.Bytes(), w.FormDataContentType(), headers, nil
}

func writePart(w *multipart.Writer, field, filename, contentType string, data []byte) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		return fmt.Errorf("encode decision request: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("encode decision request: %w", err)
	}
	return nil
}

func extensionFor(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return "jpeg"
	}
}

// systemOneResponse is the SystemOne response. Workers AI wraps the same
// object in "result".
type systemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]systemOneAnswer `json:"answers"`
	Usage   *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
	ProviderMetadata map[string]any `json:"provider_metadata"`
}

type systemOneAnswer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Probability   *float64           `json:"probability"`
	Choice        string             `json:"choice"`
	Score         *float64           `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    *float64           `json:"confidence"`
}

// workersAIEnvelope is Cloudflare's REST response envelope.
type workersAIEnvelope struct {
	Result  json.RawMessage `json:"result"`
	Success bool            `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func decodeSystemOne(ep *models.DecisionEndpoint, req Request, data []byte) (*Result, error) {
	if ep.Wire == models.DecisionWireWorkersAI {
		var env workersAIEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		}
		if !env.Success {
			var msgs []string
			for _, e := range env.Errors {
				msgs = append(msgs, fmt.Sprintf("%d: %s", e.Code, e.Message))
			}
			return nil, fmt.Errorf("%w: workers ai: %s", ErrInvalidResponse, strings.Join(msgs, "; "))
		}
		data = env.Result
	}

	var resp systemOneResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	res := &Result{Model: resp.Model, Metadata: resp.ProviderMetadata}
	if resp.Usage != nil {
		res.Usage = Usage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
	}
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
		case q.Type == Boolean && (wa.Type == "noul" || wa.Type == "boolean"):
			p := wa.Noul
			if p == nil {
				p = wa.Probability
			}
			if p == nil {
				return nil, fmt.Errorf("%w: question %q: no probability", ErrInvalidResponse, q.ID)
			}
			a, err = buildAnswer(q, "", 0, *p, nil, nil)
		case q.Type == Choice && wa.Type == "choice":
			a, err = buildAnswer(q, wa.Choice, 0, 0, wa.Probabilities, wa.Confidence)
		case q.Type == Score && wa.Type == "score" && wa.Score != nil:
			a, err = buildAnswer(q, "", *wa.Score, 0, wa.Probabilities, wa.Confidence)
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
