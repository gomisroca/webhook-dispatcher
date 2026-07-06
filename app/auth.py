from __future__ import annotations

import hmac
from fastapi import Header, HTTPException


def make_api_key_dependency(expected_key: str):
    async def verify(x_api_key: str | None = Header(default=None)) -> None:
        if not expected_key:
            return
        if not hmac.compare_digest(x_api_key or "", expected_key):
            raise HTTPException(status_code=401, detail="Missing or invalid API key")
    return verify