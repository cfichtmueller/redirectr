# redirectr

**Self-hosted HTTP redirect service — manage domain redirects via a REST API, serve them fast, and track every hit.**

redirectr runs **two servers from one binary**: a REST **API** for managing redirects, and a lean, cache-backed **redirect handler** that answers the actual browser traffic. Point a domain at the handler, configure `app.example.com → app.eu` over the API, and every request gets a `301`/`302` to the target with its path, query string, and optional UTM tags carried over. Each hit is recorded asynchronously so it never slows the redirect.

---

## Features

- **Two purpose-built servers** — a management API and a separate low-latency redirect handler, isolated so admin traffic never competes with redirect traffic.
- **Fast lookups** — an in-process LRU cache (warmed on startup, invalidated on write) serves hot domains without touching the database.
- **Path & query preserving** — `app.example.com/pricing?ref=x` redirects to `https://app.eu/pricing?ref=x`.
- **UTM injection** — attach `utm_*` parameters to a redirect and they're merged into every target URL.
- **301 or 302 per redirect**, method-aware (`POST`/`PUT`/… upgrade to `307`/`308` to preserve the method and body).
- **Loop protection** — circular and duplicate-source redirects are rejected at write time.
- **Hit tracking** — every redirect is logged to MongoDB asynchronously (best-effort, never blocking).
- **Operations-ready** — health/readiness/liveness probes, structured logging, optional Sentry, and graceful shutdown that drains in-flight work.

## How it works

```mermaid
flowchart LR
    admin([Admin]) -->|REST + bearer token| api[API server<br/>:8080]
    visitor([Visitor]) -->|HTTP request| handler[Redirect handler<br/>:8081]

    api --> mongo[(MongoDB)]
    handler -->|lookup| cache{LRU cache}
    cache -->|miss| mongo
    handler -->|301 / 302 to target| visitor
    handler -.->|async hit| stats[[stats_redirect_hits]]
    stats --- mongo
```

The **API server** owns bootstrap, authentication, and redirect CRUD. The **redirect handler** resolves the incoming `Host` header to a target, issues the redirect, and queues a hit for background recording. Both share a single MongoDB connection.

## Quick start

### 1. Run it

With Docker Compose (builds the image locally):

```yaml
# compose.yaml
services:
  mongo:
    image: mongo:7
    ports: ["27017:27017"]

  redirectr:
    build: .
    environment:
      MONGODB_URI: mongodb://mongo:27017
      MONGODB_DATABASE: redirectr
    ports:
      - "8080:8080"   # API
      - "8081:8081"   # redirect handler
    depends_on: [mongo]
```

```bash
docker compose up --build
```

Or straight from source (needs Go 1.27+ and a reachable MongoDB):

```bash
MONGODB_URI=mongodb://localhost:27017 go run .
```

### 2. Create the first user

The one-time bootstrap endpoint creates the initial user; it's a no-op once a user exists.

```bash
curl -X POST localhost:8080/api/v1/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"supersecret"}'
```

### 3. Log in

```bash
curl -X POST localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin@example.com","password":"supersecret"}'
# => {"access_token":"s-XXXXXXXX…"}
```

Send that token as `Authorization: Bearer s-XXXXXXXX…` on every management call.

### 4. Create a redirect

```bash
curl -X POST localhost:8080/api/v1/redirects \
  -H 'Authorization: Bearer s-XXXXXXXX…' \
  -H 'Content-Type: application/json' \
  -d '{
        "sourceDomain": "app.example.com",
        "targetDomain": "app.eu",
        "status": "active",
        "redirectType": "301"
      }'
```

### 5. Watch it redirect

The handler matches on the `Host` header, so you can test locally by setting it:

```bash
curl -I -H 'Host: app.example.com' localhost:8081/pricing?ref=newsletter
# HTTP/1.1 301 Moved Permanently
# Location: https://app.eu/pricing?ref=newsletter
```

In production, point `app.example.com`'s DNS at the handler and it does the rest.

## Configuration

All configuration is via environment variables. Everything has a sensible default except where noted.

| Variable           | Default                       | Description                                              |
| ------------------ | ----------------------------- | ------------------------------------------------------- |
| `API_HOST`         | `""` (all interfaces)         | Bind host for the API server                            |
| `API_PORT`         | `8080`                        | Bind port for the API server                            |
| `HANDLER_HOST`     | `""` (all interfaces)         | Bind host for the redirect handler                      |
| `HANDLER_PORT`     | `8081`                        | Bind port for the redirect handler                      |
| `MONGODB_URI`      | `mongodb://localhost:27017`   | MongoDB connection string                               |
| `MONGODB_DATABASE` | `redirectr`                   | Database name                                            |
| `SENTRY_DSN`       | `""`                          | Sentry DSN; error reporting is disabled when unset      |
| `ENVIRONMENT`      | `""`                          | Environment label reported to Sentry                    |

`MONGODB_URI` is a standard MongoDB connection string, so credentials, `authSource`, TLS, and replica sets all live in the URI:

```
mongodb://user:pass@host:27017/?authSource=admin&tls=true
```

## API reference

Base URL is the **API server** (`:8080`). All `/api/v1/redirects` endpoints require a bearer token.

### Auth

| Method & path             | Auth | Body / result                                                             |
| ------------------------- | ---- | ------------------------------------------------------------------------- |
| `POST /api/v1/bootstrap`  | —    | `{email, password}` → `204`. Creates the first user; no-op if one exists. |
| `POST /api/v1/login`      | —    | `{username, password}` → `{access_token}`. `username` is the email.       |
| `POST /api/v1/logout`     | ✓    | Invalidates the current session → `204`.                                  |
| `GET  /api/v1/me`         | —    | `{authenticated}` — reports whether the supplied token is valid.          |

### Redirects

| Method & path                  | Auth | Description                                                    |
| ------------------------------ | ---- | ------------------------------------------------------------- |
| `POST   /api/v1/redirects`     | ✓    | Create a redirect.                                            |
| `GET    /api/v1/redirects`     | ✓    | List redirects. Query: `q`, `status`, `limit`, `offset`.     |
| `GET    /api/v1/redirects/{id}`| ✓    | Fetch one redirect.                                          |
| `PUT    /api/v1/redirects/{id}`| ✓    | Update a redirect.                                           |
| `DELETE /api/v1/redirects/{id}`| ✓    | Delete a redirect → `204`.                                   |

**Redirect object**

```json
{
  "id": "…",
  "sourceDomain": "app.example.com",
  "targetDomain": "app.eu",
  "status": "active",
  "redirectType": "301",
  "utmTags": {
    "Source": "newsletter",
    "Medium": "email",
    "Campaign": "launch",
    "Term": "",
    "Content": ""
  },
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```

| Field          | Values                              | Notes                                                    |
| -------------- | ----------------------------------- | -------------------------------------------------------- |
| `sourceDomain` | a domain                            | The incoming host to match. Unique across redirects.     |
| `targetDomain` | a domain                            | Where to send visitors (always over `https`).            |
| `status`       | `active`, `inactive`, `deleted`     | Only `active` redirects are served.                      |
| `redirectType` | `301`, `302`                        | Permanent or temporary; upgraded to `307`/`308` for methods with a body. |
| `utmTags`      | object, optional                    | Any of `Source`, `Medium`, `Campaign`, `Term`, `Content`; merged into the target URL as `utm_*`. |

### Operations

| Method & path              | Auth | Description                                        |
| -------------------------- | ---- | ------------------------------------------------- |
| `GET  /internal/health`    | —    | Health incl. MongoDB check.                       |
| `GET  /internal/readiness` | —    | Readiness probe.                                  |
| `GET  /internal/liveness`  | —    | Liveness probe.                                   |
| `GET  /api/v1/cache`       | ✓    | Redirect lookup cache stats.                      |
| `POST /api/v1/cache/clear` | ✓    | Flush the lookup cache.                           |

## Development

```bash
go build ./...     # build
go test ./...      # run tests
go run .           # run locally (needs MONGODB_URI reachable)
```

## License

[MIT](LICENSE) © Christoph Fichtmüller
