"""
Dispatcher tests using a real httpx.AsyncClient pointed at httpx.MockTransport
"""
 
from __future__ import annotations
import json

import httpx
import pytest

from app.dispatcher import _backoff, _deliver_once, dispatch
from app.models import AttemptStatus, Destination, DestinationType, DispatchRequest
from app.store import EventStore


# _backoff tests

def test_backoff_doubles_each_attempt():
    assert _backoff(0, 2.0, 60.0) == 2.0
    assert _backoff(1, 2.0, 60.0) == 4.0
    assert _backoff(2, 2.0, 60.0) == 8.0

def test_backoff_capped_at_maximum():
    assert _backoff(10, 2.0, 60.0) == 60.0


# _deliver_once tests

@pytest.mark.asyncio
async def test_deliver_once_success():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200)
    
    async with httpx.AsyncClient(httpx.MockTransport(handler)) as client:
        status, error = await _deliver_once(client, "https://example.com/hook", {"foo": "bar"}, None, 5.0)

    assert status == 200
    assert error is None


@pytest.mark.asyncio
async def test_deliver_once_non_2xx_is_failure():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        status, error = await _deliver_once(client, "https://example.com/hook", {}, None, 5.0)
    assert status == 500
    assert error is not None
 
 
@pytest.mark.asyncio
async def test_deliver_once_sends_signature_when_secret_set():
    received_headers: dict = {}
 
    def handler(request: httpx.Request) -> httpx.Response:
        received_headers.update(dict(request.headers))
        return httpx.Response(200)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        await _deliver_once(client, "https://example.com/hook", {"foo": "bar"}, "mysecret", 5.0)
 
    assert "x-webhook-signature" in received_headers
    assert received_headers["x-webhook-signature"].startswith("sha256=")
 
@pytest.mark.asyncio
async def test_deliver_once_no_signature_when_no_secret():
    received_headers: dict = {}
 
    def handler(request: httpx.Request) -> httpx.Response:
        received_headers.update(dict(request.headers))
        return httpx.Response(200)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        await _deliver_once(client, "https://example.com/hook", {}, None, 5.0)
 
    assert "x-webhook-signature" not in received_headers


# full dispatch tests

def _make_store() -> EventStore:
    return EventStore(ttl_seconds=60)
 
 
def _make_request(*urls_and_types) -> DispatchRequest:
    destinations = [
        Destination(type=dtype, url=url)
        for url, dtype in urls_and_types
    ]
    return DispatchRequest(
        payload={"message": "hello", "service": "api"},
        destinations=destinations,
        event_type="test.event",
    )
 
 
@pytest.mark.asyncio
async def test_dispatch_success_all_destinations():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        req = _make_request(
            ("https://discord.com/api/webhooks/1/token", DestinationType.discord),
            ("https://hooks.slack.com/services/T/B/x", DestinationType.slack),
        )
        store = _make_store()
        resp = await dispatch(req, "evt-001", client, store, 3, 0.01, 1.0, 5.0)
 
    assert resp.event_id == "evt-001"
    assert len(resp.results) == 2
    assert all(r.status == AttemptStatus.success for r in resp.results)
    assert all(len(r.attempts) == 1 for r in resp.results)
 
 
@pytest.mark.asyncio
async def test_dispatch_retries_on_failure_then_succeeds():
    """First call returns 500, second returns 200 — should see 2 attempts, final success."""
    call_count = 0
 
    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal call_count
        call_count += 1
        return httpx.Response(200 if call_count >= 2 else 500)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        req = _make_request(("https://example.com/hook", DestinationType.generic))
        store = _make_store()
        resp = await dispatch(req, "evt-002", client, store, 3, 0.01, 1.0, 5.0)
 
    result = resp.results[0]
    assert result.status == AttemptStatus.success
    assert len(result.attempts) == 2
    assert result.attempts[0].status == AttemptStatus.failure
    assert result.attempts[1].status == AttemptStatus.success
 
 
@pytest.mark.asyncio
async def test_dispatch_gives_up_after_max_attempts():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(503)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        req = _make_request(("https://example.com/hook", DestinationType.generic))
        store = _make_store()
        resp = await dispatch(req, "evt-003", client, store, 3, 0.01, 1.0, 5.0)
 
    result = resp.results[0]
    assert result.status == AttemptStatus.failure
    assert len(result.attempts) == 3
 
 
@pytest.mark.asyncio
async def test_dispatch_fan_out_independent_per_destination():
    """A failure on one destination should not affect delivery to another."""
    call_count = {"good": 0, "bad": 0}
 
    def handler(request: httpx.Request) -> httpx.Response:
        if "good" in str(request.url):
            call_count["good"] += 1
            return httpx.Response(200)
        call_count["bad"] += 1
        return httpx.Response(500)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        req = _make_request(
            ("https://good.example.com/hook", DestinationType.generic),
            ("https://bad.example.com/hook", DestinationType.generic),
        )
        store = _make_store()
        resp = await dispatch(req, "evt-004", client, store, 2, 0.01, 1.0, 5.0)
 
    good = next(r for r in resp.results if "good" in r.url)
    bad = next(r for r in resp.results if "bad" in r.url)
    assert good.status == AttemptStatus.success
    assert bad.status == AttemptStatus.failure
 
 
@pytest.mark.asyncio
async def test_dispatch_saves_to_store():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200)
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        req = _make_request(("https://example.com/hook", DestinationType.generic))
        store = _make_store()
        await dispatch(req, "evt-005", client, store, 1, 0.01, 1.0, 5.0)
 
    record = await store.get("evt-005")
    assert record is not None
    assert record.event_type == "test.event"
 
 
@pytest.mark.asyncio
async def test_dispatch_per_destination_secret_overrides_global():
    received_sigs: list[str] = []
 
    def handler(request: httpx.Request) -> httpx.Response:
        received_sigs.append(request.headers.get("x-webhook-signature", ""))
        return httpx.Response(200)
 
    dest = Destination(
        type=DestinationType.generic,
        url="https://example.com/hook",
        secret="dest-specific-secret",
    )
    req = DispatchRequest(
        payload={"x": 1},
        destinations=[dest],
        secret="global-secret",
        event_type="test",
    )
 
    async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
        store = _make_store()
        await dispatch(req, "evt-006", client, store, 1, 0.01, 1.0, 5.0)
 
    from app.signing import sign
    body = json.dumps({"x": 1, "event_type": "test"}, default=str).encode()
    expected = sign(body, "dest-specific-secret")
    assert received_sigs[0] == expected
