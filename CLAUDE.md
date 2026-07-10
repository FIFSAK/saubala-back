# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`saubala-back` is the Go backend for a warehouse / inventory-and-contracts system
(brands → positions/lots → receipts in, releases out, against yearly contracts).
The HTTP API is served under `/api/v1`; data lives in MongoDB. The Nuxt frontend
is a separate repository at `../saubala-front` (referenced by docker-compose).

## Commands

The Makefile wraps the common workflows:

- `make run` — run the API server (`./cmd/saubala-back`)
- `make build` — `go build ./...`
- `make test` — `go test ./...`
- `make vet` — `go vet ./...`
- `make tidy` — `go mod tidy`
- `make seed` / `make seed-reset` — run the demo-data seeder (`./cmd/seed`), `-reset` wipes first
- `make import` / `make import-dry` — one-off migration of the customer's
  `поставки 2026.xlsx` workbook (`./cmd/import-xlsx`); `-dry-run` parses and
  reports without writing. Refuses to run twice (guards on existing contract
  numbers).

Run a single test:
```
go test ./internal/domain/position -run TestNew -v
```

CI (`.github/workflows/ci.yml`) gates on **gofmt** (`gofmt -l .` must be empty),
`go vet`, `go build`, and `go test ./... -race -count=1`. Always run `gofmt -w` on
touched files — unformatted code fails the build.

## Configuration

Config is loaded by `internal/config` via `godotenv` (`.env` in the working dir,
optional) then `envconfig`. Each struct has an `envconfig` prefix: `APP_*`,
`HTTP_*`, `CORS_*` (e.g. `CORS_ALLOWED_ORIGINS`), `MONGO_*`, `JWT_*`,
`SUPERADMIN_*`. Copy `.env.dist` to `.env` for local dev. Defaults make the
server runnable against `mongodb://localhost:27017` with no `.env` present.

On startup the app **seeds the super admin** (`SUPERADMIN_EMAIL`/`PASSWORD`) if
missing, and **ensures Mongo indexes** (`internal/repository/mongo/indexes.go`) —
there are no migration files; indexes are the schema contract.

## Architecture

Clean/layered architecture with **functional-options composition** at every layer.
Each layer exposes an aggregate struct (`Repositories`, `Services`, `Handlers`,
`Servers`) built by a `New(deps, ...Configuration)` constructor where each
`WithXxx()` option wires one component. The composition root is
`internal/app/init.go` (`initApp`), which builds layers in order:

```
config → store.Mongo → repositories → services → servers → handlers → RegisterHTTP
```

Layers and their dependency direction (outer depends on inner):

- **`internal/domain/<entity>/`** — pure domain. `*.go` holds the entity, its
  validation (`New(...)` constructors return plain `error`), and value types;
  `repository.go` declares the `Repository` **interface** (the persistence port)
  and the `Filter` type. No framework imports. Entities: `user`, `brand`,
  `position`, `receipt`, `release`, `contract`.
- **`internal/repository/mongo/`** — Mongo adapters implementing the domain
  `Repository` interfaces. `internal/repository/repository.go` aggregates them and
  `WithMongoStore` wires each one + calls `EnsureIndexes`.
- **`internal/service/<entity>/`** — use cases / business rules. Services depend on
  domain `Repository` interfaces (not Mongo). Cross-entity invariants live here
  (e.g. position deletion is blocked when referenced by receipts/releases/contracts).
  Services return `*web.Error` for client-facing failures.
- **`internal/handler/rest/`** — thin chi HTTP handlers: decode request DTO, call
  service, encode response DTO. Each handler has `Register(chi.Router)`.
  `internal/handler/handler.go` mounts everything under `/api/v1`.
- **`pkg/`** — reusable infra with no domain knowledge: `auth` (JWT + bcrypt),
  `store` (Mongo wrapper + `ErrorNotFound` sentinel + `IsDuplicateKey`), `server`
  (chi router + CORS + graceful shutdown), `web` (HTTP error/response/decode +
  pagination helpers), `log` (zap).

### Key conventions

- **Errors:** the service layer returns `*web.Error` (carrying an HTTP status:
  `web.BadRequest`, `NotFound`, `Conflict`, `Unprocessable`, …). Handlers call
  `web.WriteError(w, err)`, which maps `*web.Error` to its status, maps
  `store.ErrorNotFound` to 404, and everything else to a 500 without leaking
  internals. Repository "not found" surfaces as `store.ErrorNotFound`; services
  translate it with `mapNotFound`.
- **Money & mass:** monetary fields are `int64` **tiyn** (1 ₸ = 100 tiyn); mass is
  integer **grams**. Never use floats for money.
- **Stock is ledger-derived:** positions never take a direct quantity edit. Stock
  changes only through **receipts** (positive) and **releases** (negative); the
  combined history is exposed via `GET /positions/{id}/movements`. When creating a
  position with opening stock, the service applies the stock increment *before*
  writing the receipt ledger entry, and rolls back on failure, so a persisted
  receipt always corresponds to applied stock (no compensating-transaction orphans —
  there are no multi-document Mongo transactions here).
- **Auth & roles:** all routes except `POST /auth/login` require a Bearer JWT
  (`middleware.Authenticator` loads the user, rejects inactive accounts, stores it
  in context via `CurrentUser`). User-management routes additionally require
  `middleware.RequireAdmin`. Roles: `super_admin` (seed-only, never API-assignable),
  `admin`, `user`. Get the actor in a handler with `middleware.CurrentUser(ctx)`.
- **DTO mapping:** request/response structs live in the `rest` handler files (e.g.
  `createPositionRequest`, `positionResponse`, `toPositionResponse`) and are kept
  separate from domain entities — JSON shape is decoupled from storage shape.
- **Reference labels are server-enriched:** list/get responses carry
  human-readable names for referenced entities (releases → `contract_number`/
  `contract_name` + per-line `position_name`/`lot_number`; receipts and contract
  lines → `position_name`/`lot_number`; positions → `brand_name`). Services
  batch-fetch them via the repositories' `GetByIDs` (one `$in` query per
  collection, no `$lookup`); handlers merge the maps into DTOs. The frontend must
  NEVER rebuild these labels from a `page_size:100` dictionary — that cap
  silently truncates (there are 1000+ positions, 200+ contracts). For pick-lists
  the frontend uses server-side search (`q`) via `useRemoteOptions` +
  `<SelectSearch remote>`.

## Entry points

- `cmd/saubala-back/main.go` → `app.Run()` — the API server.
- `cmd/seed/main.go` — standalone demo-data seeder (brands, positions, contracts,
  releases); `-reset` wipes collections first.

## Frontend (`../saubala-front`)

The UI is a separate **Nuxt 3** repo (Vue 3 + Pinia + Tailwind), rendered
client-side only (`routeRules: { '/**': { ssr: false } }` in `nuxt.config.ts` —
a global `ssr: false` crashes `nuxt dev` on Nuxt 3.21 + Vite 7). Commands:
`npm install`, `npm run dev` (:3000), `npm run build`, `npx nuxi typecheck`.

- **API access:** `composables/useApi.ts` wraps `$fetch` with the base URL
  (`runtimeConfig.public.apiBase`, default `http://localhost:8080/api/v1`,
  overridable via `NUXT_PUBLIC_API_BASE`) and injects `Authorization: Bearer`. It
  normalises the backend `{ "error": ... }` envelope into `ApiError` and, on 401,
  clears the session and hard-redirects to `/login`. The browser origin must be in
  the backend's `CORS_ALLOWED_ORIGINS`.
- **Auth/session:** `stores/auth.ts` (Pinia) keeps `token`+`user` in
  `localStorage`; `middleware/auth.global.ts` redirects unauthenticated users to
  `/login` and gates `/users` to admins. Login via `POST /auth/login`.
- **Lists:** every list page uses `composables/useList.ts` (pagination + `q` /
  filters / `sort` / `order`, with a request-sequence guard against stale
  responses). Pages share one shape: filter toolbar + `<DataTable>` + create/edit
  `<AppModal>` (+ an `<AppDrawer>` for position movements / contract progress).
- **Money & dates:** the API sends `int64` tiyn and RFC3339; `utils/format.ts`
  converts (`formatTenge`, `inputToTiyn`, `formatDate`, …). `<MoneyInput>` binds a
  tiyn `number` while displaying tenge.
- **Layout:** `pages/` (login, index dashboard, brands, positions, receipts,
  contracts, releases, users); `layouts/` (`default` app shell, `auth`); shared
  components are auto-imported (`DataTable`, `Pagination`, `AppModal`, `AppDrawer`,
  `Badge`, `Field`, `SearchInput`, `ToastHost`, `ConfirmHost`, `AppSidebar` /
  `AppTopbar`). `stores/ui.ts` drives toasts + confirm dialogs.

**Design rules (deliberate — keep them):** this is a dense, utilitarian tool, not
a landing page. IBM Plex Sans + IBM Plex Mono (mono for numbers/IDs/money), a warm
"paper" palette with a single pine-green accent — **no** slate/violet (Tailwind's
default palette is fully replaced in `tailwind.config.ts`), hairline borders
instead of shadowed cards, 2–3px radii, lucide SVG icons (never emoji), and
concrete Russian copy.

## Docker & CI

- **Backend image:** multi-stage `Dockerfile` builds static (`CGO_ENABLED=0`)
  `server` + `seed` binaries onto `alpine`; config comes from env (no `.env` baked
  in). `HEALTHCHECK` hits `/healthz`.
- **`docker-compose.yml`** (project `saubala`): `mongo` + `backend`; profile
  `seed` runs `/app/seed -reset`; profile `full` also builds the frontend from
  `../saubala-front` (browser-facing `NUXT_PUBLIC_API_BASE=http://localhost:8080/api/v1`).
  Usage: `docker compose up -d --build`, `--profile seed run --rm seed`,
  `--profile full up`. The env values in compose are dev defaults — override
  `JWT_SECRET` / `SUPERADMIN_PASSWORD` / `CORS_ALLOWED_ORIGINS` for prod.
- **CI:** each repo has `.github/workflows/ci.yml`. Backend gates on
  gofmt / vet / build / `test -race`; frontend on `npm ci` / `nuxi typecheck` /
  `build`. On `main`, both build and push images to GHCR.
