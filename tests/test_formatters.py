from app.formatters import format_discord, format_generic, format_payload, format_slack


PAYLOAD = {"message": "Deployment complete", "service": "api", "version": "1.4.2"}


class TestDiscordFormatter:
    def test_returns_embeds_key(self):
        result = format_discord(PAYLOAD, "deploy.success")
        assert "embeds" in result
        assert len(result["embeds"]) == 1

    def test_event_type_becomes_title(self):
        embed = format_discord(PAYLOAD, "deploy.success")["embeds"][0]
        assert embed["title"] == "deploy.success"

    def test_fallback_title_when_no_event_type(self):
        embed = format_discord(PAYLOAD, None)["embeds"][0]
        assert embed["title"] == "Webhook Event"

    def test_message_field_becomes_description(self):
        embed = format_discord(PAYLOAD, None)["embeds"][0]
        assert embed["description"] == "Deployment complete"

    def test_remaining_fields_become_embed_fields(self):
        embed = format_discord(PAYLOAD, None)["embeds"][0]
        field_names = [f["name"] for f in embed["fields"]]
        assert "service" in field_names
        assert "version" in field_names
        # "message" should not appear as a field
        assert "message" not in field_names

    def test_long_values_are_truncated(self):
        long_payload = {"key": "x" * 2000}
        embed = format_discord(long_payload, None)["embeds"][0]
        assert len(embed["fields"][0]["value"]) <= 1024
 
    def test_max_25_fields_enforced(self):
        big_payload = {f"key_{i}": f"val_{i}" for i in range(30)}
        embed = format_discord(big_payload, None)["embeds"][0]
        assert len(embed["fields"]) <= 25


class TestSlackFormatter:
    def test_returns_blocks_key(self):
        result = format_slack(PAYLOAD, "deploy.success")
        assert "blocks" in result
 
    def test_event_type_becomes_header_block(self):
        blocks = format_slack(PAYLOAD, "deploy.success")["blocks"]
        assert blocks[0]["type"] == "header"
        assert blocks[0]["text"]["text"] == "deploy.success"
 
    def test_no_header_when_no_event_type(self):
        blocks = format_slack(PAYLOAD, None)["blocks"]
        assert all(b["type"] != "header" for b in blocks)
 
    def test_payload_fields_become_section_blocks(self):
        blocks = format_slack(PAYLOAD, None)["blocks"]
        section_blocks = [b for b in blocks if b["type"] == "section"]
        assert len(section_blocks) == len(PAYLOAD)
 
    def test_ends_with_divider(self):
        blocks = format_slack(PAYLOAD, "deploy.success")["blocks"]
        assert blocks[-1]["type"] == "divider"
 
    def test_max_50_blocks_enforced(self):
        big_payload = {f"key_{i}": f"val_{i}" for i in range(60)}
        blocks = format_slack(big_payload, "event")["blocks"]
        assert len(blocks) <= 50


class TestGenericFormatter:
    def test_passthrough_with_event_type(self):
        result = format_generic({"foo": "bar"}, "test.event")
        assert result["foo"] == "bar"
        assert result["event_type"] == "test.event"
 
    def test_passthrough_without_event_type(self):
        result = format_generic({"foo": "bar"}, None)
        assert result == {"foo": "bar"}
        assert "event_type" not in result
 
    def test_does_not_mutate_original_payload(self):
        payload = {"foo": "bar"}
        format_generic(payload, "event")
        assert "event_type" not in payload


class TestFormatPayload:
    def test_routes_to_correct_formatter(self):
        discord_result = format_payload("discord", PAYLOAD, "test")
        assert "embeds" in discord_result
 
        slack_result = format_payload("slack", PAYLOAD, "test")
        assert "blocks" in slack_result
 
        generic_result = format_payload("generic", PAYLOAD, "test")
        assert "event_type" in generic_result
 
    def test_unknown_type_falls_back_to_generic(self):
        result = format_payload("unknown_future_type", {"foo": "bar"}, "test")
        assert result["foo"] == "bar"
