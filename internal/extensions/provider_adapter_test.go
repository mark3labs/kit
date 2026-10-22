package extensions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func TestProviderAdapterPreflightAndCredentialRedaction(t *testing.T) {
	var requests int
	def := ProviderDef{ID: "fake", AllowUnknownModels: true, Create: func(ProviderCreateRequest) ProviderCreateResult {
		return ProviderCreateResult{Runtime: ProviderRuntime{Stream: func(ProviderRequest, func(ProviderEvent) bool) ProviderResult {
			requests++
			return ProviderResult{Error: &ProviderError{Code: "auth", Message: "bad transient-key"}}
		}}}
	}}
	model, close, err := NewProviderModel(context.Background(), def, ProviderCreateRequest{Model: "echo", APIKey: "transient-key"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	_, err = model.Generate(context.Background(), fantasy.Call{Prompt: fantasy.Prompt{{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{fantasy.FilePart{MediaType: "image/png", Data: []byte{1}}}}}})
	if err == nil || requests != 0 {
		t.Fatalf("preflight err=%v requests=%d", err, requests)
	}
	_, err = model.Generate(context.Background(), fantasy.Call{})
	if err == nil || strings.Contains(err.Error(), "transient-key") || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("unsafe error=%v", err)
	}
	var code interface{ ExtensionErrorCode() string }
	if !errors.As(err, &code) || code.ExtensionErrorCode() != "auth" {
		t.Fatalf("code=%v", err)
	}
	if _, err := model.GenerateObject(context.Background(), fantasy.ObjectCall{}); err == nil {
		t.Fatal("object operation accepted")
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}
