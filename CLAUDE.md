# GOTH Template

Starter template for Effiware GOTH-stack apps (Go + Templ + HTMX + Tailwind + Alpine).
Postgres (sqlc + goose, migrations run at startup) holds all state; Redis (optional,
nil-safe) backs sessions and caches; OTel provides traces/metrics; viper config with
`GOTH_TEMPLATE_*` env overrides.

## Commands

- `make prep` — install Go tools + npm deps, seed `.env` / `config.yaml`
- `make air` — infra containers + templ watcher + hot-reloading server
- `make test` / `make test-race` — tests (coverage excludes generated code)
- `make sqlc` / `make templ-gen` / `make swag` — regenerate after editing
  `internal/db/queries/`, `.templ` files, or swagger annotations
- `make db-migrate` / `make db-migrate-down` — manual goose cycle

## Conventions

- Keep comments and docs as compact as possible; comment only what the code cannot say.
- Never hand-edit generated files (`*_templ.go`, sqlc output in `internal/db/`,
  `internal/docs/`) — edit the source and regenerate.
- Boot wiring lives in `cmd/server/bootstrap.go`; dependencies flow through
  `server.Options` — no globals.
- Keep the htmx `responseHandling` meta in `internal/views/index.templ` in sync with
  `hda.WithHTMLFallback`; swapped error fragments must re-carry the target's id.
- The clicks demo (`internal/clicks`, migration 003, click routes/views) only shows the
  wiring — forks delete it (see README "Forking this template").
