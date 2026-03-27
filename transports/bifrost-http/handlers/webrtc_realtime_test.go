package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestResolveRealtimeSDPTarget_BaseRouteRequiresProviderPrefix(t *testing.T) {
	_, _, _, err := resolveRealtimeSDPTarget("/v1/realtime", []byte(`{"model":"gpt-4o-realtime-preview"}`))
	if err == nil {
		t.Fatal("expected provider/model validation error")
	}
	if err.Error == nil || err.Error.Message != "session.model must use provider/model on /v1 realtime routes" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestResolveRealtimeSDPTarget_BaseRouteNormalizesModel(t *testing.T) {
	provider, model, normalized, err := resolveRealtimeSDPTarget("/v1/realtime", []byte(`{"model":"openai/gpt-4o-realtime-preview","voice":"alloy"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != schemas.OpenAI {
		t.Fatalf("expected provider %s, got %s", schemas.OpenAI, provider)
	}
	if model != "gpt-4o-realtime-preview" {
		t.Fatalf("unexpected normalized model: %s", model)
	}

	var root map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(normalized, &root); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal normalized session: %v", unmarshalErr)
	}
	var sessionModel string
	if unmarshalErr := json.Unmarshal(root["model"], &sessionModel); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal model: %v", unmarshalErr)
	}
	if sessionModel != "gpt-4o-realtime-preview" {
		t.Fatalf("unexpected marshaled model: %s", sessionModel)
	}
}

func TestResolveRealtimeSDPTarget_OpenAIRouteDefaultsProvider(t *testing.T) {
	provider, model, _, err := resolveRealtimeSDPTarget("/openai/v1/realtime", []byte(`{"model":"gpt-4o-realtime-preview"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider != schemas.OpenAI {
		t.Fatalf("expected provider %s, got %s", schemas.OpenAI, provider)
	}
	if model != "gpt-4o-realtime-preview" {
		t.Fatalf("unexpected model: %s", model)
	}
}

func TestNewRealtimeRelayContextCopiesValuesWithoutRequestCancellation(t *testing.T) {
	requestCtx, requestCancel := schemas.NewBifrostContextWithCancel(context.Background())
	requestCtx.SetValue(schemas.BifrostContextKeyHTTPRequestType, schemas.RealtimeRequest)
	requestCtx.SetValue(schemas.BifrostContextKeyIntegrationType, "openai")
	requestCtx.SetValue(schemas.BifrostContextKeyGovernanceVirtualKeyID, "vk_test")

	relayCtx, relayCancel := newRealtimeRelayContext(requestCtx)
	defer relayCancel()

	requestCancel()

	select {
	case <-requestCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected request context to be cancelled")
	}

	select {
	case <-relayCtx.Done():
		t.Fatal("relay context should outlive cancelled request context")
	default:
	}

	if got := relayCtx.Value(schemas.BifrostContextKeyHTTPRequestType); got != schemas.RealtimeRequest {
		t.Fatalf("request type = %v, want %v", got, schemas.RealtimeRequest)
	}
	if got := relayCtx.Value(schemas.BifrostContextKeyIntegrationType); got != "openai" {
		t.Fatalf("integration type = %v, want %q", got, "openai")
	}
	if got := relayCtx.Value(schemas.BifrostContextKeyGovernanceVirtualKeyID); got != "vk_test" {
		t.Fatalf("virtual key id = %v, want %q", got, "vk_test")
	}
}

func TestParseRealtimeEventPreservesExtraParams(t *testing.T) {
	event, err := schemas.ParseRealtimeEvent([]byte(`{"type":"conversation.item.truncate","item_id":"item_123","content_index":0,"audio_end_ms":640}`))
	if err != nil {
		t.Fatalf("ParseRealtimeEvent() error = %v", err)
	}

	var itemID string
	if err := json.Unmarshal(event.ExtraParams["item_id"], &itemID); err != nil {
		t.Fatalf("json.Unmarshal(item_id) error = %v", err)
	}
	if itemID != "item_123" {
		t.Fatalf("item_id = %q, want %q", itemID, "item_123")
	}

	var contentIndex int
	if err := json.Unmarshal(event.ExtraParams["content_index"], &contentIndex); err != nil {
		t.Fatalf("json.Unmarshal(content_index) error = %v", err)
	}
	if contentIndex != 0 {
		t.Fatalf("content_index = %d, want 0", contentIndex)
	}
}
