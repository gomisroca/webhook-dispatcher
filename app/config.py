from __future__ import annotations

import os
from dataclasses import dataclass

@dataclass(frozen=True)
class Settings:
    port: int
    api_key: str

    # How many times to attempt delivery before giving up
    max_attemps: int 
    # Base delay for exponential backoff
    backoff_base_seconds: float
    # Hard cap on the backoff delay
    backoff_max_seconds: float
    # Per-destination HTTP timeout
    delivery_timeout_seconds: float
    # How long to keep delivery recoverds in memory before evicting
    record_ttl_seconds: float

def load_settings() -> Settings:
    return Settings(
        port=int(os.getenv("PORT", "8080")),
        api_key=os.getenv("API_KEY", ""),
        max_attemps=int(os.getenv("MAX_ATTEMPTS", "4")),
        backoff_base_seconds=float(os.getenv("BACKOFF_BASE_SECONDS", "2.0")),
        backoff_max_seconds=float(os.getenv("BACKOFF_MAX_SECONDS", "60.0")),
        delivery_timeout_seconds=float(os.getenv("DELIVERY_TIMEOUT_SECONDS", "10.0")),
        record_ttl_seconds=float(os.getenv("RECORD_TTL_SECONDS", str(60 * 60 * 24))),
    )