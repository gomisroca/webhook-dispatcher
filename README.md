# Webhook Dispatcher (Python)

A standalone microservice that accepts a JSON payload and a list of
destinations, then fans out delivery to all of them concurrently - with
per-destination formatting, HMAC signing, automatic retries, and a
delivery history you can query after the fact.

The "webhook dispatcher" pattern solves a real problem: when something
happens in your system (an order ships, a deploy finishes, a user signs
up), you often want to notify several places at once - a Discord channel,
a Slack workspace, a custom endpoint, a data pipeline. Without a
dispatcher, every caller has to know about every destination, handle
retries, and reformat the payload for each. With a dispatcher, you POST
one event and it handles everything downstream.

## Running it

```bash
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8080
```

### Tests

```bash
pytest -v
```

48 tests covering signing, each formatter, the full retry/backoff/fan-out
dispatcher logic (using `httpx.MockTransport` - no real network calls),
and the complete API via FastAPI's `TestClient`.

### Docker

```bash
docker build -t webhook-dispatcher .
docker run -p 8080:8080 \
  -e API_KEY=your-secret \
  -e MAX_ATTEMPTS=4 \
  webhook-dispatcher
```

## Config

All env vars are optional - the service runs out of the box with no setup.

| Variable                   | Default   | Description                                                     |
| -------------------------- | --------- | --------------------------------------------------------------- |
| `PORT`                     | `8080`    | HTTP port                                                       |
| `API_KEY`                  | _(empty)_ | If set, required via `X-API-Key` on all routes except `/health` |
| `MAX_ATTEMPTS`             | `4`       | How many times to attempt delivery before giving up             |
| `BACKOFF_BASE_SECONDS`     | `2`       | Starting backoff delay - doubles each retry (2s → 4s → 8s → …)  |
| `BACKOFF_MAX_SECONDS`      | `60`      | Hard cap on backoff delay                                       |
| `DELIVERY_TIMEOUT_SECONDS` | `10`      | Per-destination HTTP timeout                                    |
| `RECORD_TTL_SECONDS`       | `86400`   | How long delivery records stay queryable (default 24 hours)     |

## API

### `POST /dispatch`

Fan out a payload to one or more destinations.

**Request body:**

```json
{
  "payload": {
    "message": "Deployment complete",
    "service": "api",
    "version": "1.4.2"
  },
  "destinations": [
    { "type": "discord", "url": "https://discord.com/api/webhooks/..." },
    { "type": "slack", "url": "https://hooks.slack.com/services/..." },
    { "type": "generic", "url": "https://your-endpoint.example.com/hook" }
  ],
  "event_type": "deploy.success",
  "secret": "optional-hmac-signing-secret"
}
```

`event_type` is optional but recommended - it becomes the Discord embed
title and Slack header block. `secret` is optional - if provided, every
outgoing request gets an `X-Webhook-Signature: sha256=<hex>` header so
receivers can verify the payload came from you. Per-destination secrets
are also supported (add `"secret": "..."` inside a destination object) and
override the global secret for that destination.

**Response `200 OK`:**

```json
{
  "event_id": "8403a1a4f022ffed37da4622",
  "results": [
    {
      "destination_type": "discord",
      "url": "https://discord.com/api/webhooks/...",
      "status": "success",
      "attempts": [
        {
          "attempt_number": 1,
          "timestamp": "2026-07-05T07:22:02Z",
          "status": "success",
          "http_status": 200,
          "error": null
        }
      ]
    }
  ]
}
```

Note: the endpoint always returns `200` as long as the request was valid

- delivery failures are reported inside `results[].status`, not as HTTP
  error codes. This is intentional: the dispatcher accepted and processed
  your request; whether a downstream destination was reachable is a
  separate concern from whether _you_ got a valid response.

### `GET /events/{event_id}`

Fetch the delivery record for a specific event by the `event_id` returned
from `/dispatch`. Returns `404` if the record has expired or never existed.

### `GET /events`

List all delivery records still within the TTL window.

### `GET /health`

Liveness check - `{"status": "ok"}`. Always open, no API key required.

## Destination types

### `discord`

Formats the payload as a Discord embed. The `event_type` becomes the embed
title; any `message`/`text`/`description` field in the payload becomes the
embed description; all remaining fields become inline embed fields. Respects
Discord's field limits (max 25 fields, 256-char names, 1024-char values).

### `slack`

Formats the payload as Slack Block Kit. The `event_type` becomes a header
block; each payload field becomes a section block with Markdown formatting.
Respects Slack's 50-block limit.

### `generic`

Passes the payload through as-is, adding an `event_type` field if one was
provided. Use this for any custom endpoint, data pipeline, or service that
expects plain JSON.

## Verifying signatures on the receiving end

```python
import hashlib, hmac

def verify(body: bytes, secret: str, signature_header: str) -> bool:
    expected = "sha256=" + hmac.new(
        secret.encode(), body, hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(expected, signature_header)
```

```js
import crypto from "crypto";

function verify(body, secret, signatureHeader) {
  const expected =
    "sha256=" + crypto.createHmac("sha256", secret).update(body).digest("hex");
  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(signatureHeader),
  );
}
```

## Architecture

```
POST /dispatch
      │
      ▼
  validate request (Pydantic)
      │
      ├── destination 1 ──► format (Discord/Slack/generic)
      │                  ──► sign (HMAC-SHA256, if secret set)
      │                  ──► deliver with retries + backoff
      │
      ├── destination 2 ──► (same, concurrently via asyncio.gather)
      │
      └── destination N ──► (same)
              │
              ▼
      record all attempts in EventStore
              │
              ▼
      return DispatchResponse (all results, win or lose)
```

Key files:

- **`app/formatters.py`** - one function per destination type. Adding a new
  type means adding one function and one entry in `FORMATTERS`; nothing else
  changes.
- **`app/dispatcher.py`** - fan-out via `asyncio.gather`, retry loop with
  `_backoff()`, per-attempt recording. The core of the service.
- **`app/signing.py`** - HMAC-SHA256 sign/verify, mirroring what GitHub and
  Stripe use for their webhook signatures.
- **`app/store.py`** - in-memory delivery history with TTL-based eviction.
  In-memory by design - keeps the service self-contained and
  dependency-free. A Redis or database backend would be the natural next
  step for persistence across restarts.

## Possible next steps

- **Go port** - same service translated to Go, using goroutines +
  `sync.WaitGroup` for fan-out instead of `asyncio.gather`. The formatter
  dispatch table becomes a `map[string]FormatterFunc`, and the retry loop
  runs inside a goroutine rather than an async function.
- **Persistent store** - back `EventStore` with Redis or a database so
  delivery history survives restarts.
- **More destination types** - PagerDuty, Teams, email. Each is a new
  function in `formatters.py` and a new enum value in `models.py`.
- **Async dispatch** - return the `event_id` immediately and deliver in the
  background, rather than waiting for all deliveries (including retries) to
  complete before responding. Makes sense once retries can span minutes.
