from __future__ import annotations

import importlib
import json
from unittest.mock import patch

from fastapi.testclient import TestClient
import httpx

def _build_app(monkeypatch, **env):
    for k, v in env.items():
        monkeypatch.setenv(k, v)
    import app.config
    import app.main
    importlib.reload(app.config)
    importlib.reload(app.main)
    return app.main.app

def _always_200(request: httpx.Request) -> httpx.Response:
    return httpx.Response(200)

def _always_500(request: httpx.Request) -> httpx.Response:
    return httpx.Response(500)


VALID_BODY = {
    "payload": {"message": "hello", "service": "api"},
    "destinations": [
        {"type": "generic", "url": "https://example.com/hook"}
    ],
    "event_type": "test.event",
}


class TestHealth:
    def test_health_always_open(self, monkeypatch):
        app = _build_app(monkeypatch, API_KEY="secret")
        with TestClient(app) as client:
            assert client.get("/health").status_code == 200


class TestDispatch:
    def test_successful_dispatch(self, monkeypatch):
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1")
        with patch("app.dispatcher.httpx.AsyncClient.post",
                   return_value=httpx.Response(200)):
            with TestClient(app) as client:
                resp = client.post("/dispatch", json=VALID_BODY)
        assert resp.status_code == 200
        body = resp.json()
        assert "event_id" in body
        assert len(body["results"]) == 1
        assert body["results"][0]["status"] == "success"

    def test_dispatch_with_mock_transport(self, monkeypatch):
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1", BACKOFF_BASE_SECONDS="0.01")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(_always_200)
            )
            resp = client.post("/dispatch", json=VALID_BODY)
        assert resp.status_code == 200

    def test_all_failures_still_returns_200_with_failure_status(self, monkeypatch):
        """Dispatch itself succeeds (we accepted the request); delivery
        failures are reported in the result body, not as HTTP errors."""
        app = _build_app(monkeypatch, MAX_ATTEMPTS="2", BACKOFF_BASE_SECONDS="0.01")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(_always_500)
            )
            resp = client.post("/dispatch", json=VALID_BODY)
        assert resp.status_code == 200
        assert resp.json()["results"][0]["status"] == "failure"

    def test_discord_destination_gets_embed_format(self, monkeypatch):
        received_bodies: list[dict] = []

        def capture(request: httpx.Request) -> httpx.Response:
            received_bodies.append(json.loads(request.content))
            return httpx.Response(200)
        
        body = {**VALID_BODY, "destinations": [
            {"type": "discord", "url": "https://discord.com/api/webhooks/1/token"}
        ]}
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(capture)
            )
            client.post("/dispatch", json=body)

        assert "embeds" in received_bodies[0]

    def test_slack_destination_gets_blocks_format(self, monkeypatch):
        received_bodies: list[dict] = []

        def capture(request: httpx.Request) -> httpx.Response:
            received_bodies.append(json.loads(request.content))
            return httpx.Response(200)

        body = {**VALID_BODY, "destinations": [
            {"type": "slack", "url": "https://hooks.slack.com/services/T/B/x"}
        ]}
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(capture)
            )
            client.post("/dispatch", json=body)

        assert "blocks" in received_bodies[0]

    def test_missing_destinations_returns_422(self, monkeypatch):
        app = _build_app(monkeypatch)
        with TestClient(app) as client:
            resp = client.post("/dispatch", json={
                "payload": {"x": 1},
                "destinations": [],
            })
        assert resp.status_code == 422

    def test_api_key_required_when_set(self, monkeypatch):
        app = _build_app(monkeypatch, API_KEY="secret123")
        with TestClient(app) as client:
            resp = client.post("/dispatch", json=VALID_BODY)
            assert resp.status_code == 401

            resp = client.post("/dispatch", json=VALID_BODY, headers={"X-API-Key": "secret123"})
            assert resp.status_code == 200

class TestEvents:
    def test_get_event_after_dispatch(self, monkeypatch):
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(_always_200)
            )
            dispatch_resp = client.post("/dispatch", json=VALID_BODY)
            event_id = dispatch_resp.json()["event_id"]

            get_resp = client.get(f"/events/{event_id}")
        assert get_resp.status_code == 200
        assert get_resp.json()["event_id"] == event_id

    def test_get_missing_event_returns_404(self, monkeypatch):
        app = _build_app(monkeypatch)
        with TestClient(app) as client:
            resp = client.get("/events/missing")
        assert resp.status_code == 404

    def test_list_events_includes_recent_dispatch(self, monkeypatch):
        app = _build_app(monkeypatch, MAX_ATTEMPTS="1")
        with TestClient(app) as client:
            client.app.state.http_client = httpx.AsyncClient(
                transport=httpx.MockTransport(_always_200)
            )
            client.post("/dispatch", json=VALID_BODY)
            events = client.get("/events").json()
        assert len(events) >= 1



