from __future__ import annotations
import datetime
from enum import Enum
from typing import Any

from pydantic import BaseModel, HttpUrl, field_validator

class DestinationType(str, Enum):
    discord = "discord"
    slack = "slack"
    generic = "generic"

class Destination(BaseModel):
    type: DestinationType
    url: HttpUrl
    # Optional per destination signing secret
    secret: str | None = None

class DispatchRequest(BaseModel):
    # Arbitraty JSON payload
    payload: dict[str, Any]
    destinations: list[Destination]
    # Optional HMAC secret used to sign outgoing requests
    secret: str | None = None
    # Optional caller-supplied event type string included in the signature header
    event_type: str | None = None

    @field_validator("destinations")
    @classmethod
    def at_least_one_destination(cls, v: list) -> list:
        if not v:
            raise ValueError("Must have at least one destination")
        return v
    
class AttemptStatus(str, Enum):
    success = "success"
    failure = "failure"
    pending = "pending"

class DeliveryAttempt(BaseModel):
    attempt_number: int
    timestamp: datetime
    status: AttemptStatus
    http_status: int | None = None
    error: str | None = None

class DestinationResult(BaseModel):
    destination_type: DestinationType
    url: str
    status: AttemptStatus
    attempts: list[DeliveryAttempt]

class DispatchResponse(BaseModel):
    event_id: str
    results: list[DestinationResult]

class EventRecord(BaseModel):
    """Stored in memory so callers can query delivery status after the fact"""
    event_id: str
    event_type: str | None
    payload: dict[str, Any]
    created_at: datetime
    results: list[DestinationResult]