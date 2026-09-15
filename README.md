# Feature Flag Service

Production-ready Go REST API for storing and evaluating feature flags (global and per-user), with Postgres persistence and Redis cache-aside. Designed for DigitalOcean App Platform.

## API

### `POST /v1/flag`

Create a global flag (`user` empty) or a per-user override.

```json
{
  "user": "",
  "flagname": "dark-mode",
  "enabled": true
}
```

Responses: `201` with `{ "flagname" }`, `400` invalid flag name, `409` flag exists.

### `GET /v1/flag?user=&flagname=`

Evaluate whether a flag is enabled for a user. User override wins; otherwise the global flag is used. `404` if neither exists.

### `GET /health`

Liveness probe for App Platform.

## Local development

```bash
docker compose up --build
```

Or run dependencies and the binary:

```bash
docker compose up -d postgres redis
export DATABASE_URL='postgres://flags:flags@localhost:5432/flags?sslmode=disable'
export REDIS_URL='redis://localhost:6379/0'
go run ./cmd/server
```

Example:

```bash
curl -s -X POST localhost:8080/v1/flag \
  -H 'content-type: application/json' \
  -d '{"user":"","flagname":"beta","enabled":false}'

curl -s -X POST localhost:8080/v1/flag \
  -H 'content-type: application/json' \
  -d '{"user":"alice","flagname":"beta","enabled":true}'

curl -s 'localhost:8080/v1/flag?user=alice&flagname=beta'
```

## Tests

```bash
go test ./...
```

With local Postgres and Redis running (for example via `docker compose up -d postgres redis`):

```bash
export DATABASE_URL='postgres://flags:flags@localhost:5432/flags?sslmode=disable'
export REDIS_URL='redis://localhost:6379/0'
go test -tags=integration ./internal/integration/...
```

## Deploy to DigitalOcean App Platform

1. Push this repo to GitHub and set `services[0].github.repo` in [`.do/app.yaml`](.do/app.yaml) to `your-org/your-repo` (branch defaults to `mainline`).
2. Create the app from the spec:

```bash
doctl apps create --spec .do/app.yaml
```

Or in the DigitalOcean UI: **Apps → Create → App Spec** and paste [`.do/app.yaml`](.do/app.yaml).

The spec provisions:

- a web service built from the `Dockerfile`
- managed Postgres (`DATABASE_URL`)
- managed Redis (`REDIS_URL`)
- health checks on `/health`

Migrations apply automatically on process start.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | no | `8080` | Listen port |
| `DATABASE_URL` | yes | — | Postgres connection URL |
| `REDIS_URL` | yes | — | Redis connection URL |
| `CACHE_TTL_SECONDS` | no | `60` | Cache TTL for evaluated flags |
