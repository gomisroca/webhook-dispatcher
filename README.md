# Webhook Dispatcher (Go)

A Go port of [`Webhook Dispatcher (Python)`](https://github.com/gomisroca/webhook-dispatcher/tree/python).
HTTP API, same algorithm - swap one for the other and nothing downstream
changes. The interesting part is how the same ideas look in Go's
concurrency model vs Python's.

## Running it

```bash
go run .
```

```bash
curl -X POST http://localhost:8080/dispatch \
  -H "Content-Type: application/json" \
  -d '{
    "payload": {"message": "deploy finished", "version": "2.0.1"},
    "destinations": [
      {"type": "discord", "url": "https://discord.com/api/webhooks/..."},
      {"type": "slack",   "url": "https://hooks.slack.com/services/..."},
      {"type": "generic", "url": "https://your-endpoint.example.com/hook"}
    ],
    "event_type": "deploy.success",
    "secret": "optional-signing-secret"
  }'
```

### Tests

```bash
go test ./...
go test -race ./...   # proves the fan-out is concurrency-safe
```

### Docker

```bash
docker build -t webhook-dispatcher-go .
docker run -p 8080:8080 -e API_KEY=secret webhook-dispatcher-go
```

## Config

| Variable                 | Default | Description                                                 |
| ------------------------ | ------- | ----------------------------------------------------------- |
| PORT                     | 8080    | HTTP port                                                   |
| API_KEY                  | (empty) | If set, required via X-API-Key on all routes except /health |
| MAX_ATTEMPTS             | 4       | Delivery attempts before giving up                          |
| BACKOFF_BASE_SECONDS     | 2       | Starting retry delay, doubles each attempt (2s, 4s, 8s...)  |
| BACKOFF_MAX_SECONDS      | 60      | Hard cap on retry delay                                     |
| DELIVERY_TIMEOUT_SECONDS | 10      | Per-destination HTTP timeout                                |
| RECORD_TTL_SECONDS       | 86400   | How long delivery records stay queryable (24h)              |

## What's different from the Python version, and why

**Fan-out: goroutines + sync.WaitGroup vs asyncio.gather**

Both fan out to all destinations concurrently, but the mechanics differ:

Python's asyncio.gather schedules coroutines on one event loop thread.
They interleave at await points - cooperative multitasking. Go's goroutines
run on real OS threads across multiple cores - no GIL, no cooperative
scheduling. Each goroutine is truly independent.

The pre-allocated results slice (indexed by goroutine position) means no
mutex is needed when collecting results - each goroutine writes only to its
own slot. This is a common Go pattern; the Python version doesn't need it
because asyncio.gather returns results in order anyway.

**Retry backoff: time.Sleep in a goroutine vs await asyncio.sleep**

In Python, await asyncio.sleep yields control back to the event loop - a
plain time.sleep would block the entire thread and all other requests. In
Go, time.Sleep inside a goroutine just blocks that goroutine. The runtime
parks it and runs other goroutines on the available threads. No different
API needed.

**Store locking: sync.RWMutex vs asyncio.Lock**

Python's asyncio.Lock is cooperative (only matters at await points). Go's
sync.RWMutex is preemptive - the scheduler can switch goroutines at any
instruction. Uses RLock for reads (multiple goroutines simultaneously) and
full Lock for writes. go test -race verifies this is correct.

**No Pydantic - explicit validation in handlers**

Python's Pydantic validates request bodies automatically and returns 422
with structured error details. Go's encoding/json decodes the body and the
handler manually checks required fields. More code, more control.

**Single static binary**

go build produces one self-contained binary with no runtime dependency.
The Dockerfile run stage is FROM alpine:3.20 with just the binary - no
interpreter, no startup overhead.
