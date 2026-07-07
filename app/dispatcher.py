"""
The core of the service: takes a DispatchRequest, fans out to all
destinations concurrently, retries failures with exponential backoff, and
records every attempt.
 
Concurrency model: asyncio.gather fires all destination deliveries at
once. Each destination gets its own retry loop running concurrently with
every other destination's retry loop. A failure at one destination (or its
retries) doesn't delay delivery to others.
"""

from __future__ import annotations
import asyncio
import datetime
import json
import logging

import httpx

from app.formatters import format_payload
from app.models import AttemptStatus, DeliveryAttempt, Destination, DestinationResult, DispatchRequest, DispatchResponse, EventRecord
from app.signing import sign
from app.store import EventStore

logger = logging.getLogger(__name__)

def _backoff(attempt: int, base: float, maximum: float) -> float:
    """Exponential backoff"""
    return min(base * (2 ** attempt), maximum)

async def _deliver_once(client: httpx.AsyncClient, url: str, body: dict, secret: str | None, timeout: float) -> tuple[int | None, str | None]:
    """Attempts a single HTTP POST. Returns (http_status, error_message)."""

    raw = json.dumps(body, default=str).encode()
    headers = {"Content-Type": "application/json"}
    if secret:
        headers["X-Webhook-Signature"] = sign(raw, secret)

    try:
        resp = await client.post(url, content=raw, headers=headers, timeout=timeout)
        if resp.is_success:
            return resp.status_code, None
        return resp.status_code, f"Non-2xx response: {resp.status_code}"
    except httpx.TimeoutException:
        return None, "Timed out"
    except httpx.HTTPError as e:
        return None, str(e)
    
async def _deliver_with_retries(
    client: httpx.AsyncClient,
    destination: Destination,
    body: dict,
    secret: str | None,
    max_attempts: int,
    backoff_base: float,
    backoff_max: float,
    delivery_timeout: float,
) -> DestinationResult:
    """Retries max_attempts times with exponential backoff"""
    url = str(destination.url)
    effective_secret = destination.secret or secret
    attempts: list[DeliveryAttempt] = []

    for attempt_num in range(1, max_attempts + 1):
        http_status, error = _deliver_once(client, url, body, effective_secret, delivery_timeout)

        status = AttemptStatus.success if error is None else AttemptStatus.failure
        attempts.append(DeliveryAttempt(
            attempt_number=attempt_num,
            timestamp=datetime.now(datetime.timezone.utc),
            status=status,
            http_status=http_status,
            error=error,
        ))

        if error is None:
            logger.info("Delivered to %s on attempt %d", url, attempt_num)
            break

        logger.warning("Delivery attempt %d to %s failed: %s", attempt_num, url, error)

        if attempt_num < max_attempts:
            delay = _backoff(attempt_num - 1, backoff_base, backoff_max)
            logger.info("Retrying %s in %.1f seconds", url, delay)
            await asyncio.sleep(delay)
    
    final_status = attempts[-1].status
    return DestinationResult(
        destination_type=destination.type,
        url=url,
        status=final_status,
        attempts=attempts,
    )

async def dispatch(
    request: DispatchRequest,
    event_id: str,
    client: httpx.AsyncClient,
    store: EventStore,
    max_attempts: int,
    backoff_base: float,
    backoff_max: float,
    delivery_timeout: float,
) -> DispatchResponse:
    """
    Fan-out: all destinations are delivered concurrently via asyncio.gather.
    Each gets its own formatted body.
    """

    async def deliver_to(destination: Destination) -> DestinationResult:
        body = format_payload(destination.type.value, request.payload, request.event_type)
        return await _deliver_with_retries(
            client=client,
            destination=destination,
            body=body,
            secret=request.secret,
            max_attempts=max_attempts,
            backoff_base=backoff_base,
            backoff_max=backoff_max,
            delivery_timeout=delivery_timeout,
        )
    
    results = await asyncio.gather(*[deliver_to(d) for d in request.destinations])

    record = EventRecord(
        event_id=event_id,
        event_type=request.event_type,
        payload=request.payload,
        created_at=datetime.now(datetime.timezone.utc),
        results=results,
    )
    await store.save(record)

    return DispatchResponse(event_id=event_id, results=list(results))
