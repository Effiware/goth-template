# 🚀 GOTH Stack Template

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://go.dev/doc/go1.26)
[![Templ](https://img.shields.io/badge/Templ-0.3.1020-red?style=flat-square)](https://templ.guide)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-3.4.19-38B2AC?style=flat-square&logo=tailwind-css)](https://tailwindcss.com)
[![HTMX](https://img.shields.io/badge/HTMX-2.0.10-purple?style=flat-square)](https://htmx.org)
[![Alpine.js](https://img.shields.io/badge/Alpine.js-3.16.1-2D3441?style=flat-square)](https://alpinejs.dev)

Starter template for type-safe, server-rendered web applications on Go + Templ + HTMX +
Tailwind + Alpine.js, wired up the way Effiware production apps are: Postgres for state,
Redis for sessions and caches, OpenTelemetry for traces and metrics, and the app itself
stateless.

## ✨ What's in the box

- **📜 Type-safe templates** — [Templ](https://templ.guide), compile-time checked
- **🎨 HDA + JSON API** — the same state served as hypermedia and as JSON
- **🐘 Postgres** — [pgx](https://github.com/jackc/pgx) pool, [goose](https://github.com/pressly/goose)
  migrations embedded and run at startup, [sqlc](https://sqlc.dev)-generated queries
- **⚡ Redis** — session store and the cache wrappers in [`utils/`](utils/README.md); optional
- **⚙️ Config** — [viper](https://github.com/spf13/viper): YAML file + `GOTH_TEMPLATE_*` env overrides
- **🔭 Observability** — OTLP traces, Prometheus metrics on `/metrics`, JSON logs carrying `trace_id`
- **🩺 Ops** — `/ping` liveness, `/readyz` readiness, SIGTERM drain, graceful shutdown
- **🔥 Hot reload** — [Air](https://github.com/air-verse/air) + the templ proxy
- **📘 Swagger** — [swag](https://github.com/swaggo/swag)-generated docs at `/swagger`

## 🚀 Quick start

### Prerequisites

- Go v1.26, npm v11.4, node v24.4 (or just Docker 28.1)
- GNU Make (recommended)

Tools (templ, air, swag, sqlc, goose) are Go tool dependencies — `make prep` installs them.

### Run it

```bash
make prep          # tools, npm deps, .env and config.yaml from the examples
make air           # infra containers + templ watcher + hot-reloading server
```

Open `http://localhost:8080`. Click the button (HDA), then hit the same counter over JSON:

```bash
curl http://localhost:8080/api/v1/clicks
curl -X POST http://localhost:8080/api/v1/clicks/increment
```

`make air` starts Postgres, Redis and Jaeger via `docker-up-infra` and tears them down on
exit. For the whole stack in containers instead: `make docker-build && make docker-up`.

## 🗺️ Layout

```
cmd/server/          main.go (lifecycle) + bootstrap.go (boot* wiring — put boot logic here)
internal/
  clicks/            demo domain package: Postgres state read through a Redis cache
  config/            viper config, defaults and validation
  db/                pgx pool, tx + advisory-lock helpers, migrations/, queries/, sqlc output
  server/            HTTP: routes, middlewares/, hda/ (hypermedia), api/ (JSON), session/
  services/          background jobs: Servicer + periodic/scheduled runners, gated fleet-wide
  static/            css/, img/, js/ (htmx + alpine, vendored — no CDN)
  views/             .templ pages and components/
  version/           ldflags-injected version + the OTel instrumentation scope name
utils/               resilience primitives: retry, circuit, caches (see utils/README.md)
```

## ⚙️ Configuration

`config.yaml` (copy of [`config.example.yaml`](config.example.yaml)), with every key
overridable by env — `GOTH_TEMPLATE_SERVER_PORT`, `GOTH_TEMPLATE_DATABASE_URL`, … (dots
become underscores). `.env` is only for docker-compose port mapping and the goose CLI.

Redis is optional: with `redis.url` empty the session store and caches switch off and the
app keeps serving. Tracing is off when `otlp.url` is empty and no `OTEL_*` env is set.

## 🛠️ Make targets

| Target | Does |
|---|---|
| `prep` | install tools + npm deps, seed `.env` and `config.yaml` |
| `air` | infra up, templ watch, hot-reloading server, infra down on exit |
| `build` / `build-local` | Tailwind + Go build / Go only |
| `test` / `test-race` | tests with coverage (generated code excluded) / race detector |
| `templ-gen` / `sqlc` / `swag` | regenerate templates / queries / API docs |
| `db-migrate` / `db-migrate-down` | manual goose cycle (the app migrates itself at startup) |
| `docker-build` / `docker-up` / `docker-up-infra` / `docker-down` | containers |

## 🧭 Forking this template

1. Rename the module (`go.mod`), `version.ServiceName`, and the compose/image names.
2. Delete the demo: `internal/clicks`, `services/demo_service.go`, migration `003`,
   `queries/clicks.sql`, the click routes, views and `api.Clicks`.
3. Prune `internal/db/migrations/001`–`002` down to the tables you actually want, then
   `make sqlc`.
4. Drop what you don't need — Redis, the session store, `services/`, `utils/` — each is
   nil-safe or self-contained.

## 📝 Notes

### Port already taken

```bash
sudo lsof -i -P | grep LISTEN | grep :<PORT>   # find the PID
sudo kill -9 <PID>
```

### Hot reloading

Air rebuilds Tailwind + Go on change; `templ generate --watch` regenerates templates and
notifies its proxy, which reloads the browser. There is an in-depth explanation in
[this](https://medium.com/ostinato-rigore/go-htmx-templ-tailwind-complete-project-setup-hot-reloading-2ca1ba6c28be)
article.

### Remove a Go tool

```bash
go get -tool <package>@none
```
