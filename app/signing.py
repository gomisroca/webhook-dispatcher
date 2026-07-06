"""
HMAC-SHA256 signing for outgoing webhook deliveries.
 
The signature is sent as an X-Webhook-Signature header so receivers can
verify the payload actually came from this service and wasn't tampered
with in transit. The format mirrors what GitHub and Stripe use:
 
    X-Webhook-Signature: sha256=<hex_digest>
 
Receivers verify it by computing the same HMAC over the raw request body
using their copy of the secret, and comparing with constant-time equality.
"""

from __future__ import annotations

import hashlib
import hmac

def sign(payload: bytes, secret: str) -> str:
    """Returns the signature header value, e.g. 'sha256=abc123...'"""

    digest = hmac.new(secret.encode(), payload, hashlib.sha256).hexdigest()
    return f"sha256={digest}"

def verify(payload: bytes, secret: str, signature: str) -> bool:
    """Constant-time verification"""

    expected = sign(payload, secret)
    return hmac.compare_digest(expected, signature)