package models

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openai"
)

func TestCreateCustomProvider_ReasoningEffort(t *testing.T) {
	cases := []struct {
		name       string
		level      ThinkingLevel
		wantEffort *openai.ReasoningEffort
		wantNil    bool
	}{
		{
			name:       "thinking off yields nil options",
			level:      ThinkingOff,
			wantNil:    true,
		},
		{
			name:       "empty thinking level yields nil options",
			level:      "",
			wantNil:    true,
		},
		{
			name:       "thinking none",
			level:      ThinkingNone,
			wantEffort: new(openai.ReasoningEffortNone),
		},
		{
			name:       "thinking minimal",
			level:      ThinkingMinimal,
			wantEffort: new(openai.ReasoningEffortMinimal),
		},
		{
			name:       "thinking low",
			level:      ThinkingLow,
			wantEffort: new(openai.ReasoningEffortLow),
		},
		{
			name:       "thinking medium",
			level:      ThinkingMedium,
			wantEffort: new(openai.ReasoningEffortMedium),
		},
		{
			name:       "thinking high",
			level:      ThinkingHigh,
			wantEffort: new(openai.ReasoningEffortHigh),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ProviderConfig{
				ModelString:   "custom/test-model",
				ProviderURL:   "http://localhost:8000/v1",
				ThinkingLevel: tc.level,
			}

			result, err := CreateProvider(context.Background(), cfg)
			if err != nil {
				t.Fatalf("CreateProvider failed: %v", err)
			}

			if tc.wantNil {
				if result.ProviderOptions != nil {
					t.Fatalf("expected nil ProviderOptions, got %+v", result.ProviderOptions)
				}
				return
			}

			if result.ProviderOptions == nil {
				t.Fatal("expected non-nil ProviderOptions")
			}

			rawOpts, ok := result.ProviderOptions[openai.Name]
			if !ok {
				t.Fatalf("expected provider options under key %q", openai.Name)
			}

			opts, ok := rawOpts.(*openai.ProviderOptions)
			if !ok {
				t.Fatalf("expected *openai.ProviderOptions, got %T", rawOpts)
			}

			if opts.ReasoningEffort == nil {
				t.Fatal("expected non-nil ReasoningEffort")
			}

			if *opts.ReasoningEffort != *tc.wantEffort {
				t.Errorf("ReasoningEffort = %q, want %q", *opts.ReasoningEffort, *tc.wantEffort)
			}
		})
	}
}

func TestCreateCustomProvider_TransmitsReasoningEffortInRequest(t *testing.T) {
	cases := []struct {
		name          string
		level         ThinkingLevel
		expectEffort  bool
		wantEffortVal string
	}{
		{
			name:          "medium thinking transmits reasoning_effort medium",
			level:         ThinkingMedium,
			expectEffort:  true,
			wantEffortVal: "medium",
		},
		{
			name:          "thinking off does not transmit reasoning_effort",
			level:         ThinkingOff,
			expectEffort:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedBody map[string]any

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_ = json.Unmarshal(bodyBytes, &capturedBody)

				resp := map[string]any{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": 1234567890,
					"model":   "test-model",
					"choices": []map[string]any{
						{
							"index": 0,
							"message": map[string]any{
								"role":    "assistant",
								"content": "OK",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]any{
						"prompt_tokens":     5,
						"completion_tokens": 1,
						"total_tokens":      6,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			cfg := &ProviderConfig{
				ModelString:   "custom/test-model",
				ProviderURL:   server.URL + "/v1",
				ThinkingLevel: tc.level,
			}

			result, err := CreateProvider(context.Background(), cfg)
			if err != nil {
				t.Fatalf("CreateProvider failed: %v", err)
			}

			call := fantasy.Call{
				Prompt:          fantasy.Prompt{fantasy.NewUserMessage("hello")},
				ProviderOptions: result.ProviderOptions,
			}

			_, err = result.Model.Generate(context.Background(), call)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			effortVal, exists := capturedBody["reasoning_effort"]
			if tc.expectEffort {
				if !exists {
					t.Fatalf("expected reasoning_effort in request body, got body: %+v", capturedBody)
				}
				if effortVal != tc.wantEffortVal {
					t.Errorf("reasoning_effort = %v, want %v", effortVal, tc.wantEffortVal)
				}
			} else {
				if exists {
					t.Fatalf("expected no reasoning_effort in request body, got: %v", effortVal)
				}
			}
		})
	}
}
