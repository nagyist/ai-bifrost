package handlers

import (
	"encoding/json"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestResolveRealtimeClientSecretTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		route        schemas.RealtimeSessionRoute
		body         []byte
		wantProvider schemas.ModelProvider
		wantModel    string
		wantErr      bool
	}{
		{
			name:         "base route with session model",
			route:        schemas.RealtimeSessionRoute{Path: "/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets},
			body:         []byte(`{"session":{"model":"openai/gpt-4o-realtime-preview"}}`),
			wantProvider: schemas.OpenAI,
			wantModel:    "gpt-4o-realtime-preview",
		},
		{
			name:         "base route with top level model",
			route:        schemas.RealtimeSessionRoute{Path: "/v1/realtime/sessions", EndpointType: schemas.RealtimeSessionEndpointSessions},
			body:         []byte(`{"model":"openai/gpt-4o-realtime-preview"}`),
			wantProvider: schemas.OpenAI,
			wantModel:    "gpt-4o-realtime-preview",
		},
		{
			name:         "openai alias uses bare model",
			route:        schemas.RealtimeSessionRoute{Path: "/openai/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets, DefaultProvider: schemas.OpenAI},
			body:         []byte(`{"session":{"model":"gpt-4o-realtime-preview"}}`),
			wantProvider: schemas.OpenAI,
			wantModel:    "gpt-4o-realtime-preview",
		},
		{
			name:    "base route rejects bare model",
			route:   schemas.RealtimeSessionRoute{Path: "/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets},
			body:    []byte(`{"session":{"model":"gpt-4o-realtime-preview"}}`),
			wantErr: true,
		},
		{
			name:    "missing model",
			route:   schemas.RealtimeSessionRoute{Path: "/openai/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets, DefaultProvider: schemas.OpenAI},
			body:    []byte(`{"session":{}}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotProvider, gotModel, _, err := resolveRealtimeClientSecretTarget(tt.route, tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRealtimeClientSecretTarget() error = %v", err)
			}
			if gotProvider != tt.wantProvider {
				t.Fatalf("provider = %q, want %q", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Fatalf("model = %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}

func TestResolveRealtimeClientSecretTarget_NormalizesModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		route     schemas.RealtimeSessionRoute
		body      string
		wantModel string // bare model expected in normalized body
	}{
		{
			name:      "session.model provider prefix stripped",
			route:     schemas.RealtimeSessionRoute{Path: "/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets},
			body:      `{"session":{"model":"openai/gpt-4o-realtime-preview","voice":"alloy"}}`,
			wantModel: "gpt-4o-realtime-preview",
		},
		{
			name:      "top-level model provider prefix stripped",
			route:     schemas.RealtimeSessionRoute{Path: "/v1/realtime/sessions", EndpointType: schemas.RealtimeSessionEndpointSessions},
			body:      `{"model":"openai/gpt-4o-realtime-preview"}`,
			wantModel: "gpt-4o-realtime-preview",
		},
		{
			name:      "bare model unchanged on alias route",
			route:     schemas.RealtimeSessionRoute{Path: "/openai/v1/realtime/client_secrets", EndpointType: schemas.RealtimeSessionEndpointClientSecrets, DefaultProvider: schemas.OpenAI},
			body:      `{"session":{"model":"gpt-4o-realtime-preview"}}`,
			wantModel: "gpt-4o-realtime-preview",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, normalizedBody, err := resolveRealtimeClientSecretTarget(tt.route, []byte(tt.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var root map[string]json.RawMessage
			if unmarshalErr := json.Unmarshal(normalizedBody, &root); unmarshalErr != nil {
				t.Fatalf("failed to unmarshal normalized body: %v", unmarshalErr)
			}

			// Check session.model if present
			if sessionJSON, ok := root["session"]; ok {
				var session map[string]json.RawMessage
				if unmarshalErr := json.Unmarshal(sessionJSON, &session); unmarshalErr != nil {
					t.Fatalf("failed to unmarshal session: %v", unmarshalErr)
				}
				if modelJSON, ok := session["model"]; ok {
					var model string
					if unmarshalErr := json.Unmarshal(modelJSON, &model); unmarshalErr != nil {
						t.Fatalf("failed to unmarshal session.model: %v", unmarshalErr)
					}
					if model != tt.wantModel {
						t.Fatalf("session.model = %q, want %q", model, tt.wantModel)
					}
				}
			}

			// Check top-level model if present
			if modelJSON, ok := root["model"]; ok {
				var model string
				if unmarshalErr := json.Unmarshal(modelJSON, &model); unmarshalErr != nil {
					t.Fatalf("failed to unmarshal model: %v", unmarshalErr)
				}
				if model != tt.wantModel {
					t.Fatalf("model = %q, want %q", model, tt.wantModel)
				}
			}
		})
	}
}

func TestIsJSONContentType(t *testing.T) {
	t.Parallel()

	if !isJSONContentType("application/json; charset=utf-8") {
		t.Fatal("expected application/json content type to pass")
	}
	if !isJSONContentType("application/vnd.openai+json") {
		t.Fatal("expected +json content type to pass")
	}
	if isJSONContentType("text/plain") {
		t.Fatal("expected text/plain content type to fail")
	}
}
