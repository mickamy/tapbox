# tapbox example

A minimal note-taking app that demonstrates tapbox tracing across HTTP, gRPC, and PostgreSQL.

## Architecture

```
curl ──► tapbox :8080 (HTTP proxy) ──► api :3000
                                         │
              tapbox :9090 (gRPC proxy) ◄─┘
                │
                ▼
            backend :50051
                │
              tapbox :5433 (SQL proxy) ◄─┘
                │
                ▼
            postgres :5432
```

All traffic flows through tapbox proxies, producing a unified trace for each request.

## Quick start

```bash
docker compose up --build -d

# Open tapbox UI
open http://localhost:3080

# Send some requests
curl -s http://localhost:8080/notes | jq
curl -s -X POST http://localhost:8080/notes \
  -H 'Content-Type: application/json' \
  -d '{"title":"Hello","body":"from tapbox"}' | jq
curl -s http://localhost:8080/notes/1 | jq
```

Open http://localhost:3080 to see the full HTTP → gRPC → SQL trace for each request.

## Cleanup

```bash
docker compose down -v
```
