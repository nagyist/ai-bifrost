package handlers

import (
	"encoding/json"
	"testing"

	"github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestShouldAccumulateRealtimeOutput(t *testing.T) {
	provider := &openai.OpenAIProvider{}
	if !provider.ShouldAccumulateRealtimeOutput(schemas.RTEventResponseTextDelta) {
		t.Fatal("expected response.text.delta to accumulate output text")
	}
	if !provider.ShouldAccumulateRealtimeOutput(schemas.RTEventResponseAudioTransDelta) {
		t.Fatal("expected response.audio_transcript.delta to accumulate output transcript")
	}
	if provider.ShouldAccumulateRealtimeOutput(schemas.RTEventInputAudioTransDelta) {
		t.Fatal("did not expect input audio transcription delta to accumulate assistant output")
	}
}

func TestExtractRealtimeTurnSummary(t *testing.T) {
	event := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventConversationItemCreate,
		Item: &schemas.RealtimeItem{
			Content: []byte(`[{"type":"input_text","text":"hello from realtime"}]`),
		},
	}

	got := extractRealtimeTurnSummary(event, "")
	if got != "hello from realtime" {
		t.Fatalf("extractRealtimeTurnSummary() = %q, want %q", got, "hello from realtime")
	}
}

func TestFinalizedRealtimeInputSummary(t *testing.T) {
	userCreate := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventConversationItemCreate,
		Item: &schemas.RealtimeItem{
			Role:    "user",
			Content: []byte(`[{"type":"input_text","text":"hello from browser"}]`),
		},
	}
	if got := finalizedRealtimeInputSummary(userCreate); got != "hello from browser" {
		t.Fatalf("finalizedRealtimeInputSummary(user create) = %q, want %q", got, "hello from browser")
	}

	inputTranscript := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventInputAudioTransCompleted,
		ExtraParams: map[string]json.RawMessage{
			"transcript": json.RawMessage(`"spoken user turn"`),
		},
	}
	if got := finalizedRealtimeInputSummary(inputTranscript); got != "spoken user turn" {
		t.Fatalf("finalizedRealtimeInputSummary(input transcript) = %q, want %q", got, "spoken user turn")
	}

	emptyInputTranscript := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventInputAudioTransCompleted,
		ExtraParams: map[string]json.RawMessage{
			"transcript": json.RawMessage(`""`),
		},
		RawData: []byte(`{"type":"conversation.item.input_audio_transcription.completed","transcript":"","usage":{"total_tokens":11}}`),
	}
	if got := finalizedRealtimeInputSummary(emptyInputTranscript); got != realtimeMissingTranscriptText {
		t.Fatalf("finalizedRealtimeInputSummary(empty input transcript) = %q, want %q", got, realtimeMissingTranscriptText)
	}

	missingInputTranscript := &schemas.BifrostRealtimeEvent{
		Type:    schemas.RTEventInputAudioTransCompleted,
		RawData: []byte(`{"type":"conversation.item.input_audio_transcription.completed","usage":{"total_tokens":11}}`),
	}
	if got := finalizedRealtimeInputSummary(missingInputTranscript); got != realtimeMissingTranscriptText {
		t.Fatalf("finalizedRealtimeInputSummary(missing input transcript) = %q, want %q", got, realtimeMissingTranscriptText)
	}

	assistantCreate := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventConversationItemCreate,
		Item: &schemas.RealtimeItem{
			Role:    "assistant",
			Content: []byte(`[{"type":"text","text":"assistant text"}]`),
		},
	}
	if got := finalizedRealtimeInputSummary(assistantCreate); got != "" {
		t.Fatalf("finalizedRealtimeInputSummary(assistant create) = %q, want empty", got)
	}
}

func TestFinalizedRealtimeToolOutputSummary(t *testing.T) {
	event := &schemas.BifrostRealtimeEvent{
		Type: schemas.RTEventConversationItemCreate,
		Item: &schemas.RealtimeItem{
			Type:   "function_call_output",
			Output: `{"nextResponse":"tool result"}`,
		},
	}
	if got := finalizedRealtimeToolOutputSummary(event); got != `{"nextResponse":"tool result"}` {
		t.Fatalf("finalizedRealtimeToolOutputSummary() = %q, want %q", got, `{"nextResponse":"tool result"}`)
	}
}

func TestBuildRealtimeTurnPostResponseUsesFullResponseDonePayload(t *testing.T) {
	rawRequest := `{"type":"conversation.item.input_audio_transcription.completed","transcript":""}`
	rawResponse := []byte(`{
		"type":"response.done",
		"response":{
			"output":[
				{
					"id":"item_message_123",
					"type":"message",
					"content":[
						{
							"type":"audio",
							"transcript":"assistant turn text"
						}
					]
				}
			],
			"usage":{
				"total_tokens":26,
				"input_tokens":17,
				"output_tokens":9,
				"input_token_details":{
					"text_tokens":12,
					"audio_tokens":5,
					"image_tokens":0,
					"cached_tokens":4
				},
				"output_token_details":{
					"text_tokens":7,
					"audio_tokens":2
				}
			}
		}
	}`)

	resp := buildRealtimeTurnPostResponse(&openai.OpenAIProvider{}, schemas.OpenAI, "gpt-4o-realtime-preview-2025-06-03", rawRequest, rawResponse, "")
	if resp == nil || resp.ResponsesResponse == nil {
		t.Fatal("expected realtime post response to be built")
	}
	if resp.ResponsesResponse.Usage == nil || resp.ResponsesResponse.Usage.InputTokens != 17 || resp.ResponsesResponse.Usage.OutputTokens != 9 || resp.ResponsesResponse.Usage.TotalTokens != 26 {
		t.Fatalf("Usage = %+v, want input=17 output=9 total=26", resp.ResponsesResponse.Usage)
	}
	if len(resp.ResponsesResponse.Output) != 1 {
		t.Fatalf("len(Output) = %d, want 1", len(resp.ResponsesResponse.Output))
	}
	if resp.ResponsesResponse.Output[0].Content == nil || resp.ResponsesResponse.Output[0].Content.ContentStr == nil || *resp.ResponsesResponse.Output[0].Content.ContentStr != "assistant turn text" {
		t.Fatalf("Output[0].Content = %+v, want assistant turn text", resp.ResponsesResponse.Output[0].Content)
	}
	if got, ok := resp.ResponsesResponse.ExtraFields.RawRequest.(string); !ok || got != rawRequest {
		t.Fatalf("RawRequest = %#v, want %q", resp.ResponsesResponse.ExtraFields.RawRequest, rawRequest)
	}
	if got, ok := resp.ResponsesResponse.ExtraFields.RawResponse.(string); !ok || got == "" {
		t.Fatalf("RawResponse = %#v, want raw response string", resp.ResponsesResponse.ExtraFields.RawResponse)
	}
}
