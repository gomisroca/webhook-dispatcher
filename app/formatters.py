"""
Formatters translate a raw payload dict into the specific JSON shape each
destination type expects. Each formatter returns a plain dict that gets
JSON-serialised into the outgoing request body.
 
Adding a new destination type means adding one function here and one entry
in FORMATTERS, nothing else in the codebase changes.
"""

from __future__ import annotations

from typing import Any

def _truncate(text: str, limit: int) -> str:
    """Discord/Slack have field limits, so truncate if needed"""
    return text if len(text) <= limit else text[:limit - 1] + "..."

def format_discord(payload: dict[str, Any], event_type: str | None) -> dict[str, Any]:
    """
    Discord payload formatter using embeds
    https://discord.com/developers/docs/resources/channel#embed-object
    """

    title = event_type or "Webhook Event"
    description = payload.get("message") or payload.get("text") or payload.get("description") or ""

    fields = [
        {
            "name": _truncate(str(k), 256),
            "value": _truncate(str(v), 1024),
            "inline": True,
        }
        for k, v in payload.items()
        if k not in ("message", "text", "description")
    ]

    embed: dict[str, Any] = {
        "title": _truncate(title, 256),
        "color": 5793266,  # a neutral blue
        "fields": fields[:25],  # Discord hard limit
    }

    if description:
        embed["description"] = _truncate(description, 4096)

    return {"embeds", [embed]}

def format_slack(payload: dict[str, Any], event_type: str | None) -> dict[str, Any]:
    """
    Slack payload formatter using blocks
    https://api.slack.com/block-kit
    """

    blocks: list[dict[str, Any]] = []

    if event_type:
        blocks.append({
            "type": "header",
            "text": {
                "type": "plain_text",
                "text": _truncate(event_type, 150),
            },
        })

    for key, value in payload.items():
        text = f"*{key}*\n{_truncate(str(value), 2900)}"
        blocks.append({
            "type": "section",
            "text": {
                "type": "mrkdwn",
                "text": text,
            },
        })

    
    # Slack recommends a divider at the end for readability
    if blocks:
        blocks.append({"type": "divider"})

    return {"blocks": blocks[:50]} # Slack hard limit

def format_generic(payload: dict[str, Any], event_type: str | None) -> dict[str, Any]:
    """
    Passes the payload through as-is, adding "event_type" field if provided.
    """

    body = dict(payload)
    if event_type:
        body["event_type"] = event_type
    return body

FORMATTERS = {
    "discord": format_discord,
    "slack": format_slack,
    "generic": format_generic,
}

def format_payload(destination_type: str, payload: dict[str, Any], event_type: str | None) -> dict[str, Any]:
    formatter = FORMATTERS.get(destination_type, format_generic)
    return formatter(payload, event_type)