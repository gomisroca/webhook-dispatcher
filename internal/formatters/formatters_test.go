package formatters

import (
	"strings"
	"testing"
	"webhook-dispatcher/internal/models"
)

var testPayload = map[string]any{
	"message": "Deployment complete",
	"service": "api",
	"version": "1.4.2",
}

func strPtr(s string) *string { return &s }

func TestDiscord_HasEmbedsKey(t *testing.T) {
	result := FormatDiscord(testPayload, strPtr("deploy.success"))
	if _, ok := result["embeds"]; !ok {
		t.Fatal("expected embeds key")
	}
}
func TestDiscord_EventTypeBecomesTitle(t *testing.T) {
	result := FormatDiscord(testPayload, strPtr("deploy.success"))
	embeds := result["embeds"].([]any)
	embed := embeds[0].(map[string]any)
	if embed["title"] != "deploy.success" {
		t.Fatalf("expected title 'deploy.success', got %v", embed["title"])
	}
}

func TestDiscord_FallbackTitleWhenNoEventType(t *testing.T) {
	result := FormatDiscord(testPayload, nil)
	embed := result["embeds"].([]any)[0].(map[string]any)
	if embed["title"] != "Webhook Event" {
		t.Fatalf("expected fallback title, got %v", embed["title"])
	}
}

func TestDiscord_MessageBecomesDescription(t *testing.T) {
	result := FormatDiscord(testPayload, nil)
	embed := result["embeds"].([]any)[0].(map[string]any)
	if embed["description"] != "Deployment complete" {
		t.Fatalf("expected description, got %v", embed["description"])
	}
}

func TestDiscord_LongValuesTruncated(t *testing.T) {
	payload := map[string]any{"key": strings.Repeat("x", 2000)}
	result := FormatDiscord(payload, nil)
	embed := result["embeds"].([]any)[0].(map[string]any)
	fields := embed["fields"].([]map[string]any)
	if len([]rune(fields[0]["value"].(string))) > 1024 {
		t.Fatal("expected value to be truncated to 1024 chars")
	}
}

func TestDiscord_Max25Fields(t *testing.T) {
	payload := make(map[string]any)
	for i := 0; i < 30; i++ {
		payload[strings.Repeat("k", i+1)] = "v"
	}
	result := FormatDiscord(payload, nil)
	embed := result["embeds"].([]any)[0].(map[string]any)
	fields := embed["fields"].([]map[string]any)
	if len(fields) > 25 {
		t.Fatalf("expected max 25 fields, got %d", len(fields))
	}
}

func TestSlack_HasBlocksKey(t *testing.T) {
	result := FormatSlack(testPayload, strPtr("deploy.success"))
	if _, ok := result["blocks"]; !ok {
		t.Fatal("expected blocks key")
	}
}

func TestSlack_EventTypeBecomesHeaderBlock(t *testing.T) {
	result := FormatSlack(testPayload, strPtr("deploy.success"))
	blocks := result["blocks"].([]any)
	first := blocks[0].(map[string]any)
	if first["type"] != "header" {
		t.Fatalf("expected header block, got %v", first["type"])
	}
}

func TestSlack_NoHeaderWhenNoEventType(t *testing.T) {
	result := FormatSlack(testPayload, nil)
	blocks := result["blocks"].([]any)
	for _, b := range blocks {
		block := b.(map[string]any)
		if block["type"] == "header" {
			t.Fatal("expected no header block when event type is nil")
		}
	}
}

func TestSlack_EndsWithDivider(t *testing.T) {
	result := FormatSlack(testPayload, strPtr("event"))
	blocks := result["blocks"].([]any)
	last := blocks[len(blocks)-1].(map[string]any)
	if last["type"] != "divider" {
		t.Fatalf("expected last block to be divider, got %v", last["type"])
	}
}
func TestGeneric_PassthroughWithEventType(t *testing.T) {
	result := FormatGeneric(map[string]any{"foo": "bar"}, strPtr("test.event"))
	if result["foo"] != "bar" || result["event_type"] != "test.event" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestGeneric_NoEventTypeField_WhenNil(t *testing.T) {
	result := FormatGeneric(map[string]any{"foo": "bar"}, nil)
	if _, ok := result["event_type"]; ok {
		t.Fatal("expected no event_type field")
	}
}

func TestGeneric_DoesNotMutateOriginal(t *testing.T) {
	payload := map[string]any{"foo": "bar"}
	FormatGeneric(payload, strPtr("event"))
	if _, ok := payload["event_type"]; ok {
		t.Fatal("expected original payload not to be mutated")
	}
}

func TestFormat_RoutesToCorrectFormatter(t *testing.T) {
	et := "test"
	discord := Format(models.DestinationDiscord, testPayload, &et)
	if _, ok := discord["embeds"]; !ok {
		t.Fatal("expected discord to produce embeds")
	}

	slack := Format(models.DestinationSlack, testPayload, &et)
	if _, ok := slack["blocks"]; !ok {
		t.Fatal("expected slack to produce blocks")
	}

	generic := Format(models.DestinationGeneric, testPayload, &et)
	if generic["event_type"] != "test" {
		t.Fatal("expected generic to include event_type")
	}
}

func TestFormat_UnknownTypeFallsBackToGeneric(t *testing.T) {
	result := Format("unknown_future_type", map[string]any{"foo": "bar"}, nil)
	if result["foo"] != "bar" {
		t.Fatal("expected generic fallback")
	}
}