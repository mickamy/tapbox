# tapbox

A local dev tool that traces HTTP, gRPC, and SQL in a single view.

![demo](./docs/demo.gif)

## Features

- **Multi-protocol proxy** — HTTP, gRPC, Connect, and PostgreSQL
- **W3C Trace Context** — automatic `traceparent` propagation across services
- **sqlcommenter support** — correlates SQL queries with the HTTP/gRPC spans that triggered them
- **Real-time Web UI** — live trace viewer powered by Server-Sent Events
- **EXPLAIN analysis** — run `EXPLAIN` on captured SQL queries from the UI
- **YAML + CLI config** — configure via CLI flags or `.tapbox.yaml`
- **Homebrew install** — `brew install mickamy/tap/tapbox`

## Install

### Homebrew

```sh
brew install mickamy/tap/tapbox
```

### go install

```sh
go install github.com/mickamy/tapbox@latest
```

Requires Go 1.26+.

### Build from source

```sh
git clone https://github.com/mickamy/tapbox.git
cd tapbox
make install # or `make build` to just build the binary in bin/
```

## Quick start

```sh
tapbox --http-target localhost:3000
```

Open [http://localhost:3080](http://localhost:3080) to view traces.

Requests to the proxy at `:8080` are forwarded to your upstream at `localhost:3000`, and every span appears in the Web UI.

## Configuration

All flags can also be set in a `.tapbox.yaml` file (auto-loaded from the current directory, if present).

| Flag              | Default         | Description                                     |
|-------------------|-----------------|-------------------------------------------------|
| `--http-target`   | *(required)*    | Upstream HTTP server                            |
| `--http-listen`   | `:8080`         | HTTP proxy listen address                       |
| `--grpc-target`   | *(disabled)*    | Upstream gRPC server                            |
| `--grpc-listen`   | `:9090`         | gRPC proxy listen address                       |
| `--sql-target`    | *(disabled)*    | Upstream PostgreSQL server                      |
| `--sql-listen`    | `:5433`         | SQL proxy listen address                        |
| `--ui-listen`     | `:3080`         | Web UI listen address                           |
| `--max-body-size` | `65536` (64 KB) | Max request/response body capture size in bytes |
| `--max-traces`    | `1000`          | Max traces to keep in memory                    |
| `--explain-dsn`   | *(sql-target)*  | PostgreSQL DSN for EXPLAIN queries              |
| `--config`        | `.tapbox.yaml` (if present) | Path to YAML config file              |

## Architecture

```
                  ┌──────────────┐
                  │    Client    │
                  └──┬───┬───┬───┘
                     │   │   │
        ┌────────────┘   │   └────────────┐
        ▼                ▼                ▼
 ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
 │ HTTP proxy  │  │ gRPC proxy  │  │  SQL proxy  │
 │   :8080     │  │   :9090     │  │   :5433     │
 └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
        │                │                │
        ▼                ▼                ▼
    upstream         upstream        PostgreSQL

        └────────────┬────────────┘
                     ▼
              ┌───────────┐
              │ Collector │
              └─────┬─────┘
                    ▼
              ┌───────────┐
              │  Web UI   │
              │   :3080   │
              └───────────┘
```

## Example

See the [`example/`](./example/) directory for a working demo with HTTP, gRPC, and PostgreSQL backends.

## License

[MIT](./LICENSE)
