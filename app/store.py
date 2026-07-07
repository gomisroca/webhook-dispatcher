"""
In-memory store for delivery records. Lets callers query what happened to
an event after dispatch.
 
Not persisted to disk, a restart clears history.
"""

from __future__ import annotations
import asyncio
from datetime import time

from app.models import EventRecord

class EventStore:
    def __init__(self, ttl_seconds: float) -> None:
        self._records: dict[str, tuple[float, EventRecord]] = {}
        self._lock = asyncio.Lock()
        self._ttl = ttl_seconds

    async def save(self, record: EventRecord) -> None:
        async with self._lock:
            expiry = time.monotonic() + self._ttl
            self._records[record.event_id] = (expiry, record)

    async def get(self, event_id: str) -> EventRecord | None:
        async with self._lock:
            entry = self._records.get(event_id)
            if entry is None: 
                return None # If no record, return None
            expiry, record = entry
            if time.monotonic() > expiry: # If expired, remove record and return None
                del self._records[event_id]
                return None
            return record
        
    async def all(self) -> list[EventRecord]:
        now = time.monotonic()
        async with self._lock:
            stale = [k for k, (exp, _) in self._records.items() if now > exp]
            for k in stale:
                del self._records[k]
            return [record for _, record in self._records.values()]
        
    async def evict_expired(self) -> int:
        """Called by the background task to remove expired records"""
        now = time.monotonic()
        async with self._lock:
            stale = [k for k, (exp, _) in self._records.items() if now > exp]
            for k in stale:
                del self._records[k]
            return len(stale)
        
    def __len__(self) -> int:
        return len(self._records)