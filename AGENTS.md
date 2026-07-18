# ABCMovies — AGENTS.md

## Dev commands
- `go tool templ generate` — regenerate `*_templ.go` after editing `.templ` files
- `go generate ./...` — regenerate `internal/oas/` from `api/openapi.yaml` (ogen)
- `go vet ./... && go test ./...` — verify + test (no external test deps, stdlib only)
- `make generate` — all codegen: css + js + templ + ogen
- `go build -o bin/abcmovies ./cmd/`
- `go run ./cmd/ -config config.yaml` — run locally

## Architecture
- Two handlers share one `registry.Registry`:
  - `internal/web/` — HTML pages (mounted at `/`)
  - `internal/handler/` — JSON API (mounted at `/api/v1alpha` by ogen server)
- Providers implement `provider.Provider` interface; lookup via `registry.Get(tag)`
- Only stub provider wired in `providers/factory.go` (`sidecar/` exists but unused)

## Web handler layout
| Directory | Purpose |
|---|---|
| `pages/` | Full-page templates (wrapped in layouts) |
| `fragments/` | HTMX fragment templates (no layout chrome) |
| `components/` | Reusable building blocks (cards, nav, sentinel, hero, stream list) |
| `layouts/` | App shell (Base → App → content) |

Reusable components:
- `CardLink(href)` — card `<a>` wrapper with consistent styling
- `BackLink(href, label)` — blue back-link `← label`
- `Sentinel(nextURL)` — infinite scroll sentinel div
- `BackdropHero(url)` — hero section with gradient overlay + children slot
- `StreamList(streams, baseURL)` — list of stream links

## Infinite scroll pattern
- List handlers parse `limit, offset` via `parsePagination(r)`
- `HX-Request` header → return fragment template (`fragments.*`)
- Otherwise → return full page template (`pages.*`)
- Both use `@components.Sentinel(nextURL)` for the sentinel div

## Defaults
- Web page size: `const defaultPageSize = 20` in `internal/web/handler.go`
- Web max: `const maxPageSize = 100` (also used as "fetch all" for seasons/episodes)
- API: `const defaultLimit = 20` in `internal/handler/handler.go`

## Testing
- stdlib only: `testing`, `httptest` — no testify
- External test packages: `web_test`, `handler_test`
- Stub provider configured inline: `stub.New(stub.Config{Tag: "T", Service: ..., Movies: ...})`
- Error semantics: provider returns `provider.ErrNotSupported` for unimplemented methods
- Web tests: call `h.ServeHTTP(rec, req)`, check `rec.Code` and `strings.Contains(body, "text")`

## OAS codegen
- Source: `api/openapi.yaml` → target: `internal/oas/`
- Config: `.ogen.yaml` (disables webhooks, enables client request validation)
- `go generate ./...` triggers `ogen` via `//go:generate` in `generate.go`

## Docker deployment
- Multi-stage Dockerfile: `golang-base` → `dev` / `testing` / `builder-prod` → `prod`
- `deploy/dev.sh` — `docker compose --profile dev up --watch`; uses `air` for hot-reload, syncs source on file changes
- `deploy/testing.sh` — `docker compose --profile testing up`; runs `make test && make vet` inside container then exits
- `deploy/prod.sh` — `docker compose --profile prod up -d`; detached multi-stage scratch image, no dev tooling
- `compose.yaml` dev profile uses `develop.watch` (sync+rebuild triggers on go.mod/go.sum/air config changes)
