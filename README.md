# caddy-utm-tracker

A Caddy v2 HTTP middleware plugin that transparently captures UTM parameters and marketing attribution data from incoming requests and forwards them as events to [OpenPanel](https://openpanel.dev). Zero code changes required on any upstream service.

## Why?

Every service behind Caddy gets marketing attribution tracking for free — no SDK integration, no frontend JavaScript, no code changes. The proxy sees the UTM params first, fires the event asynchronously, and passes the request through untouched.

## Features

- **UTM Parameter Extraction** — Captures `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, plus click IDs (`gclid`, `fbclid`, `msclkid`) and custom params
- **Visitor Identity** — Cookie-based visitor tracking with configurable first-party cookies
- **Session Tracking** — Sliding-expiration session cookies
- **Async Event Dispatch** — Events are buffered and sent in batches; never blocks the request
- **Bot Filtering** — Built-in bot User-Agent detection with configurable patterns
- **UTM Stripping** — Optionally remove tracking params before forwarding to upstream
- **Path Filtering** — Track/ignore specific paths with glob patterns
- **Configurable** — All settings available via Caddyfile or JSON config

## Installation

### Using xcaddy

```bash
xcaddy build --with github.com/xor22h/caddy-utm-tracker
```

### Using Docker

```dockerfile
FROM caddy:builder AS builder
RUN xcaddy build --with github.com/xor22h/caddy-utm-tracker

FROM caddy:latest
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

For local development:

```dockerfile
FROM caddy:builder AS builder
COPY . /src/caddy-utm-tracker
RUN xcaddy build --with github.com/xor22h/caddy-utm-tracker=/src/caddy-utm-tracker

FROM caddy:latest
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

## Configuration

### Caddyfile

```caddyfile
{
    order utm_tracker before reverse_proxy
}

example.com {
    utm_tracker {
        # Required
        client_id     "{env.OPENPANEL_CLIENT_ID}"
        client_secret "{env.OPENPANEL_CLIENT_SECRET}"

        # Optional — OpenPanel endpoint (default: https://api.openpanel.dev)
        endpoint      "https://api.openpanel.dev"

        # Cookie settings
        cookie_name     "_vid"          # default: _vid
        cookie_domain   ".example.com"  # default: request host
        cookie_max_age  365d            # default: 365 days
        cookie_secure   true            # default: true
        cookie_http_only true           # default: true
        cookie_same_site lax            # default: lax

        # Session cookie
        session_cookie  "_sid"          # default: _sid
        session_max_age 30m             # default: 30 minutes

        # Behavior
        strip_params     true           # default: false — remove UTM params from upstream URL
        only_with_params false          # default: false — only track when UTM params present
        track_methods    GET            # default: GET

        # Path filtering
        ignore_paths /api/* /static/* /_next/* /favicon.ico
        track_paths  /*

        # Batching
        batch_size     50              # default: 50
        flush_interval 5s              # default: 5s

        # Custom params to track (in addition to built-in UTMs)
        extra_params ref via partner_id

        # Additional bot patterns (built-in list always active)
        bot_patterns MyCustomBot AnotherBot
    }

    reverse_proxy upstream:8080
}
```

### Tracked Parameters

| Query Parameter | OpenPanel Property |
|----------------|-------------------|
| `utm_source` | `utm_source` |
| `utm_medium` | `utm_medium` |
| `utm_campaign` | `utm_campaign` |
| `utm_term` | `utm_term` |
| `utm_content` | `utm_content` |
| `ref` | `referral_code` |
| `gclid` | `gclid` |
| `fbclid` | `fbclid` |
| `msclkid` | `msclkid` |

Additional parameters can be tracked using `extra_params`.

### Duration Format

Duration values support Go duration syntax (`5s`, `30m`, `1h`) plus day notation (`365d`).

## How It Works

```
Request arrives
    |
    +-- Is bot? --> skip tracking, pass through
    |
    +-- Method not in track_methods? --> pass through
    |
    +-- Path matches ignore_paths? --> pass through
    |
    +-- Path doesn't match track_paths? --> pass through
    |
    +-- Extract/create visitor cookie (_vid)
    +-- Extract/create session cookie (_sid)
    +-- Extract UTM + tracking params
    +-- Build event, push to async buffer
    |
    +-- If strip_params: remove UTM params from URL
    |
    +-- Pass request to next handler (NEVER BLOCKED)
```

Events are batched and flushed to OpenPanel either when the batch size is reached or the flush interval elapses. On shutdown, remaining events are flushed.

## Development

### Build

```bash
go build ./...
```

### Test

```bash
go test -race -cover ./...
```

### Run locally with Docker Compose

```bash
export OPENPANEL_CLIENT_ID=your_client_id
export OPENPANEL_CLIENT_SECRET=your_client_secret
docker compose up --build
```

Then test with:

```bash
curl -v "http://localhost/page?utm_source=test&utm_medium=email&utm_campaign=launch"
```

## OpenPanel API

This plugin sends events to the [OpenPanel Track API](https://openpanel.dev/docs/api/track). Each page view is sent as:

```json
{
  "type": "track",
  "payload": {
    "name": "page_view",
    "profileId": "<visitor_id>",
    "properties": {
      "path": "/pricing",
      "referrer": "https://google.com",
      "utm_source": "twitter",
      "utm_medium": "social",
      "utm_campaign": "launch"
    }
  }
}
```

With headers:
- `openpanel-client-id` — Your OpenPanel client ID
- `openpanel-client-secret` — Your OpenPanel client secret
- `x-client-ip` — The real client IP
- `User-Agent` — The original request User-Agent

## Future Work

- Retry queue / dead-letter for failed API calls
- SPA client-side navigation tracking
- Consent management / GDPR cookie banner integration
- Multiple analytics backend support (Segment, PostHog, Plausible)
- Response-based tracking (track only 2xx responses)
- A/B test variant tracking
- Revenue / conversion event tracking

## License

MIT
