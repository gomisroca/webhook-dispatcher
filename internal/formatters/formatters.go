// Formatters translate a raw payload into a specific JSON shape for a given destination.
//
// Python used a plain dict of functions:
//	FORMATTERS = {"discord": format_discord, ...}
//
// Go expresses the same idea as a map of typed function values:
//	var formatters = map[models.DestinationType]FormatterFunc{...}

package formatters

import (
	"fmt"
	"webhook-dispatcher/internal/models"
)

type FormatterFunc func(payload map[string]any, eventType *string) map[string]any

var registry = map[models.DestinationType]FormatterFunc{}

// Route to the correct formatter, falling back to a generic formatter.
func Format(destType models.DestinationType, payload map[string]any, eventType *string) map[string]any {
	if fn, ok := registry[destType]; ok {
		return fn(payload, eventType)
	}
	return FormatGeneric(payload, eventType)
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}

func strVal(v any) string {
	return fmt.Sprintf("%v", v)
}

func FormatGeneric(payload map[string]any, eventType *string) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		out[k] = v
	}
	if eventType != nil && *eventType != "" {
		out["event_type"] = *eventType
	}
	return out
}

func FormatDiscord(payload map[string]any, eventType *string) map[string]any {
	title := "Webhook Event"
	if eventType != nil && *eventType != "" {
		title = *eventType
	}

	description := ""
	for _, key := range []string{"message", "text", "description"} {
		if v, ok := payload[key]; ok {
			description = strVal(v)
			break
		}
	}

	skipKeys := map[string]bool{"message": true, "text": true, "description": true}
	var fields []map[string]any
	for k, v := range payload {
		if skipKeys[k] || len(fields) >= 25 {
			break
		}
		fields = append(fields, map[string]any{
			"name":  truncate(k, 256),
			"value": truncate(strVal(v), 1024),
			"inline": true,
		})
	}

	embed := map[string]any{
		"title": truncate(title, 256),
		"color":  5793266,
		"fields": fields,
	}
	if description != "" {
		embed["description"] = truncate(description, 4096)
	}

	return map[string]any{"embeds": []any{embed}}
}

func FormatSlack(payload map[string]any, eventType *string) map[string]any {
	var blocks []any

	if eventType != nil && *eventType != "" {
		blocks = append(blocks, map[string]any{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": truncate(*eventType, 150),
			},
		})
	}

	for k, v := range payload {
		if len(blocks) >= 49 {
			break
		}
		text := fmt.Sprintf("*%s*\n%s", k, truncate(strVal(v), 2900))
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": text},
		})
	}

	if len(blocks) > 0 {
		blocks = append(blocks, map[string]any{"type": "divider"})
	}

	return map[string]any{"blocks": blocks}
}