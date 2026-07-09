from __future__ import annotations
from contextlib import asynccontextmanager
import secrets

from fastapi import Depends, FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
import httpx

from app.auth import make_api_key_dependency
from app.config import load_settings
from app.dispatcher import dispatch
from app.models import DispatchRequest, DispatchResponse, EventRecord
from app.store import EventStore

settings = load_settings()
store = EventStore(ttl_seconds=settings.record_ttl_seconds)

@asynccontextmanager
async def lifespan(_: FastAPI):
    if not settings.api_key:
        print("WARNING: API_KEY is not set- all endpoints are open.")
    print(
        f"Webhook Dispatcher starting on :{settings.port} "
        f"(max_attempts={settings.max_attemps}, "
        f"backoff={settings.backoff_base_seconds}s base)"
    )
    async with httpx.AsyncClient() as client:
        app.state.http_client = client
        yield

app = FastAPI(title="Webhook Dispatcher", lifespan=lifespan)

# Permissive CORS policy
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Content-Type", "X-API-Key"],
)

require_api_key = make_api_key_dependency(settings.api_key)

@app.post("/dispatch", dependencies=[Depends(require_api_key)])
async def dispatch_event(request: DispatchRequest) -> DispatchResponse:
    """
    Accept a payload and fan it out to all listed destinations concurrently.
    Each destination is formatted for its type (Discord embed, Slack blocks,
    or raw JSON), signed if a secret is provided, and delivered with
    automatic retries on failure.
    """

    event_id = secrets.token_hex(12)
    return await dispatch(
        request=request,
        event_id=event_id,
        client=app.state.http_client,
        store=store,
        max_attempts=settings.max_attempts,
        backoff_base=settings.backoff_base_seconds,
        backoff_max=settings.backoff_max_seconds,
        delivery_timeout=settings.delivery_timeout_seconds,
    )

@app.get("/events", dependencies=[Depends(require_api_key)])
async def list_events() -> list[EventRecord]:
    """Return all delivery records still within the TTL window"""

    return await store.all()

@app.get("/events/{event_id}", dependencies=[Depends(require_api_key)])
async def get_event(event_id: str) -> EventRecord:
    """Return the delivery record for a given event ID"""
    
    record = await store.get(event_id)
    if record is None:
        raise HTTPException(status_code=404, detail="Event not found")
    return record

@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}