# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run (config path is required as first arg; run from repo root — config paths are relative to cwd)
go run ./internal ./configs/config.toml

# Build (package main is internal/)
go build -o immo-lux ./internal

# Tests — PostgreSQL integration tests skip unless IMMOLUX_TEST_DATABASE_URL is set
go test ./...
go test ./internal/utils/...                       # Single package
go test ./internal/utils -run '^TestSlugify$' -v   # Single test
IMMOLUX_TEST_DATABASE_URL='postgres://user:pass@localhost:5432/immolux_test?sslmode=disable' go test ./...

# Dependencies — run `go generate` after tidy: tidy drops the go.sum entries of
# the Ent code generator, which deploy.sh's `go generate` would re-add on the server
go get -u ./... && go mod tidy

# Regenerate Ent ORM code (run after any schema change)
go generate ./internal/database/ent
```

There are two sets of integration tests: store-level ones in `internal/database` (`newTestDatabase` in `testdb_test.go`) and end-to-end HTTP ones in `internal/server/integration_test.go`, which run the real server — middleware, routes and stores — on a loopback listener. Both migrate and **truncate every table** of the database they connect to, so they refuse to run unless the database name (or, for the store tests only, the `search_path` schema) contains the word `test`, e.g. `immolux_test`. Use a dedicated database (the app user needs `CREATEDB`, or create it as a superuser with `OWNER` set to the app user). Because `go test ./...` runs packages in parallel, every such test holds a PostgreSQL advisory lock (`testDatabaseLockKey`, same value in both packages) for its duration. Unit tests in `internal/server/server_test.go` cover the limiter and client-IP resolution without a database, using `app.Test` (fake peer `0.0.0.0`, never trusted) and a real `127.0.0.1` listener (trusted loopback).

**Local setup:** requires Go 1.27.1+ (the `go` directive in `go.mod`) and PostgreSQL (developed and tested on 18). Copy `configs/config.toml.template` → `configs/config.toml` and `configs/standardUserList.json.template` → `configs/standardUserList.json` (both gitignored). For local development set `origin = "http://localhost:8080"` (the front end's dev server, `../immo-lux-front-end`); keep `port = 8082`, which the front end and `deploy.sh` expect.

## Architecture

**Entry point:** `internal/main.go` — reads config path from `os.Args[1]`, initializes DB (runs migrations + seeds users), registers routes, starts Fiber v3 server with graceful shutdown on `SIGINT`/`SIGTERM`.

**Request flow:**
```
Global middleware (getFiberApp): CORS → login rate limiter (POST /v1/api/login only)
  → Fiber Router (/v1/api prefix)
  → GetRouteContext() [always runs extractUserContext(): validates JWT + DB session if a cookie is present — public routes included]
  → handleRoute() [secured routes reject unauthenticated requests here]
  → Route handler (internal/server/routes/) [parse → call store operation → respond]
  → Domain store (internal/database/) [owner-scoped operations that own their transactions]
  → Ent ORM (internal/database/ent/client/) [auto-generated, never edit]
```

Handlers never see `*client.Tx` or any other ent type — the ORM stays behind the
`internal/database` seam (by convention; `Database.Client()` and `WithTransaction` are exported).

**Layer map:**
| Directory | Purpose |
|-----------|---------|
| `internal/server/routes/` | HTTP handlers, one file per domain; request payload structs and some response wrappers are declared here |
| `internal/database/` | Domain stores (one per entity) + `AuthOperations` — owner-scoped, transaction-owning |
| `internal/database/ent/schema/` | Ent schema definitions — edit these |
| `internal/database/ent/client/` | Auto-generated Ent code (committed) — never edit |
| `internal/models/` | Entity DTOs and shared API types |
| `internal/config/` | TOML config via Viper, struct-based with getter methods |
| `internal/server_error/` | `ServerError` type (Kind + Code + Message + Cause) |
| `internal/utils/` | JWT, image processing, location validation + slugs, session hashing |
| `internal/logger/` | zerolog wrapper with file rotation (lumberjack) |

## Key Patterns

### RouteContext

Every handler receives `*routes.RouteContext` instead of `fiber.Ctx`. It provides:
- `ctx.IsAuthenticated()` / `ctx.AuthError()` — auth state
- `ctx.RespondData(data)` — standardized success envelope
- `ctx.ReadBody(&dto)` — decode request body (returns a kinded 400 error on bad JSON)
- `ctx.ParseIdParam("id")` / `ctx.RequireUserId()` — return kinded errors, never respond themselves
- `ctx.RequestContext()` — the `context.Context` to pass to every store call (Fiber v3 returns `context.Background()` here, so it is not cancelled when the client disconnects)
- `ctx.ClientIP()` — normalised client address (see Authentication); use it instead of `ctx.Ctx().IP()`
- `ctx.Db()` / `ctx.ServerConfig()` / `ctx.Logger()` / `ctx.Ctx()` — pre-injected deps and the raw Fiber context

**Handlers respond with data or return an error — never both.** Error-to-HTTP
mapping happens centrally; handlers must not pick status codes.

Routes are registered in `internal/server/routing.go` via `server.Get()/Post()/Put()/Delete()` (public) or `server.SecuredGet()` etc. (blocks unauthenticated requests before the handler). Static paths like `/properties/facets` must be registered before `/properties/:id`.

### API Response Envelope

JSON responses (success and error) use the same shape:
```json
{ "success": true|false, "data": any|null, "error": { "code": "...", "message": "..." }|null }
```
Exceptions: `GET /images/:id` returns raw image bytes, and Fiber-native errors (unmatched route 404, 413 over the 10 MB `BodyLimit`) are plain Fiber responses — there is no custom Fiber `ErrorHandler`.

### Error Handling

`ServerError` carries a **Kind** that decides the HTTP status. Both pieces live in
`internal/server/server.go`: `handleServerError` (wraps non-`ServerError`s as
`SERVER_ERROR`, logs, writes the envelope) and `httpStatus` (Kind → status).
Construct errors with the kinded constructors and just `return err` from handlers:

```go
server_error.NotFound("PROPERTY_NOT_FOUND", "property not found")     // → 404
server_error.Forbidden("ACCESS_DENIED", "not your property")          // → 403
server_error.Invalid("PROPERTY_VALIDATION", "title is required")      // → 400
server_error.BadRequest("malformed body")                              // → 400, code BAD_REQUEST
server_error.Unauthorized("INVALID_CREDENTIALS", "bad credentials")   // → 401
server_error.RateLimited("RATE_LIMIT_EXCEEDED", "try again later")    // → 429
server_error.New("...", "...") / server_error.Wrap("...", "...", err) // → 500 (internal)
err.WithCause(cause)                                                   // attach cause to a kinded error
server_error.IsServerError(err, "USER_NOT_FOUND")                     // code check
```

Codes are uppercase snake_case strings. The central adapter also logs: 5xx at
Error level, 4xx at Debug — handlers don't log their own failures. The front end
maps `INVALID_CREDENTIALS` and `RATE_LIMIT_EXCEEDED` to translated messages, so
don't rename those.

### Transactions

```go
db.WithTransaction(ctx, func(txCtx context.Context, tx *client.Tx) error {
    // auto-rollback on error, auto-commit on success
})
```

Transactions are **internal to `internal/database`**: multi-step store
operations (create property, delete property + images, login) open their own
transaction; single-statement mutations (publish, revoke session, delete
contact/image) are owner-predicated `Update()/Delete().Where(id, owner)` calls
whose zero-rows outcome is classified by the shared `ownershipVerdict` helper
(`ownership.go`) into not-found vs forbidden. Exported store methods never take
a `*client.Tx`; unexported ones run inside the caller's transaction. Route
handlers never construct transactions or import ent types.

### DTOs

Entity DTOs use pointer fields (`*string`, `*int64`, etc.) so unset fields serialize as `null`; the aggregate DTOs in `internal/models/property.go` (facets, location stats, published locations) use plain values. `RecordId` is `int64`; `InvalidRecordId = -1`, `UnknownRecordId = -2`. The front end mirrors these by hand in `src/lib/types/` — keep JSON names in sync.

## Public Property API

`GET /v1/api/properties` (`routes/property_public.go`, `property_repository.go`) always forces `is_published=true` and accepts:
- Filters: `district`, `municipality`, `parish`, `propertyType`, `status` (exact canonical values, not slugs), `minPrice`/`maxPrice` (unparseable values ignored), `q` (case-insensitive substring over title, description, address and location fields)
- Paging: `limit` (default 20, no upper cap), `offset`
- `orderBy`: `price_asc|price_desc|created_asc|created_desc` (default) `|popularity|location|status` — `status` sorts in memory after loading every row (it's the `/my-properties` default)
- Returns `{properties, total}`

`GET /properties/facets` takes the same filters and returns per-dimension counts, each computed with every filter except its own. `GET /locations/stats?district=<slug>[&municipality=<slug>]` takes **slugs** and resolves them to canonical names (the front end's district/municipality landing pages depend on this); `GET /locations/published` lists districts/municipalities that have published properties, with slugs, for the sitemap.

`GET /properties/:id` (and its images) does **not** check `is_published`, and increments `view_count` on every hit — including the panel's edit form loading it.

## Authentication

JWT (HS256, claims include `userId` + `sessionId`) is stored in the `session_token` cookie (HttpOnly, `SameSite=Strict`, `Path=/`, `Secure` from `[security].cookie_secure`). The Session row stores an opaque SHA-256 token that is **not** derived from the JWT; requests are validated by:
1. `utils.ValidateJWT` — HMAC signature + expiry only (the issuer is set but not verified)
2. Looking up the Session by the JWT's `sessionId` claim — must have `is_active=true` and `expires_at > now`

Because `extractUserContext` runs on every request, any request carrying a cookie costs a session lookup, and an invalid cookie is cleared even on public routes.

`rememberMe=true` → 30-day JWT, session and cookie. `rememberMe=false` → 1-hour JWT but 24-hour session/cookie (`utils.CalculateExpiration`), so the effective login is 1 hour.

Login is rate limited twice:
1. **In memory, per client** — `POST /login` allows `LoginRequestLimit` (10) requests per `LoginRequestWindow` (1 min) per IP (IPv6 bucketed by /64), via Fiber's limiter in `getFiberApp` (`internal/server/server.go`). Over the limit it returns 429 with the envelope and code `RATE_LIMIT_EXCEEDED`; rejections are logged at most once a minute as a summary. This stops floods before they reach the database. It is registered after CORS so the 429 still carries CORS headers. The store is per process (prefork multiplies the limit).
2. **In the database, per `(email, ip_address, action)`** — 5 attempts per 15-minute window, 30-minute block when exceeded; fails open if the rate-limit query errors. Each new `(email, IP)` pair inserts a `rate_limits` row, which is why the in-memory limit matters. Because it is keyed by IP, it does not stop a distributed brute force against one account.

**Client IP:** `routes.ClientIP` normalises what Fiber resolves (unmaps IPv4-mapped IPv6, drops zones, falls back to the TCP peer for anything that doesn't parse as an IP) so it fits the 45-char `ip_address` columns. Without `[server].proxy_header` the client IP is the TCP peer, so behind a reverse proxy every client looks like the proxy and shares one rate-limit bucket — anyone could then block all logins; the server logs a startup warning when the header is unset and it listens on loopback. Setting it (e.g. `"X-Forwarded-For"`) makes Fiber trust that header only from loopback peers and take the rightmost valid untrusted entry. Have the proxy **overwrite** the header (`proxy_set_header X-Forwarded-For $remote_addr;`): with an appending proxy, a client can still inject entries if the proxy's own entry fails to parse (e.g. `ip:port` or `::ffff:` forms). `Forwarded` (RFC 7239) is rejected at config load.

## Ent ORM

Schema definitions live in `internal/database/ent/schema/`. After any change:

```bash
go generate ./internal/database/ent
```

This runs `go run -mod=mod entgo.io/ent/cmd/ent generate --target ./client ./schema` (`-mod=mod` may update `go.mod`/`go.sum`) and regenerates everything under `internal/database/ent/client/`. Do not edit files in `client/` directly; commit the regenerated output.

The database is **PostgreSQL** (`lib/pq`). Schema migrations run automatically at startup via `entClient.Schema.Create(ctx)` without drop options — new columns/indexes are applied, but renames and drops are not. The `db/` directory is a leftover from an old SQLite setup and is unused.

Fields marked `.Sensitive()` (e.g., `hash`, `image_data`, `session_token`) are hidden from `String()` and JSON marshaling only — **they are still selected by queries**. Image metadata queries therefore project through `imageMetadataColumns` (`property_image_repository.go`) so blobs never leave the database; only `ImagePayload` reads `image_data`.

### Data retention

`Database.Init()` starts a maintenance goroutine (`internal/database/maintenance.go`) that runs at startup and then every 24 hours, deleting records older than `DataRetention` (90 days): sessions that expired or were invalidated before the cutoff, auth logs created before it, and rate-limit records whose window started before it. Each table's delete has its own timeout; failures are logged at Error and don't skip the other tables, and each run ends with a Debug summary. `Database.Close()` cancels the job (rolling back an in-flight delete) and waits for it. Deleted rows free space for reuse inside PostgreSQL, but the files on disk don't shrink without `VACUUM FULL`.

## Image Handling

Images are stored as BLOBs in the `property_images` table (Sensitive field). Upload is `POST /properties/:id/images`, multipart field `image` plus optional `displayOrder`:
- At most `MaxImagesPerProperty` (20) images per property → 400 `IMAGE_LIMIT_REACHED`. The upload transaction takes the property's row lock (`lockOwned`, an owner-predicated update of `updated_at`) before counting, so concurrent uploads cannot exceed the cap. `DeleteProperty` takes the same lock, so an upload in progress either finishes first (and its image is deleted) or then finds the property gone. The panel UI allows 10, deleting removed images before uploading new ones
- Max 10 MB (Fiber's `BodyLimit` applies to the whole multipart body, so near-limit files get a non-enveloped 413)
- Format detected by magic bytes; supported input: JPEG, PNG, WebP, TIFF, BMP (AVIF/HEIC/HEIF explicitly rejected)
- Always converted to JPEG quality 85
- Resized to max 1920×1920 (aspect ratio preserved)

Served via `GET /v1/api/images/:id` with the stored `content_type` and `Cache-Control: public, max-age=31536000`.

The image store has two faces: metadata queries (gallery listing, ownership
resolution, display order) project only metadata columns, while `ImagePayload`
is the single query that reads the blob. Keep new queries on the right side of
that split.

## Location Validation

Portugal's administrative divisions (districts → municipalities → parishes) are embedded as a JSON file at `internal/utils/data/portugal-admin-divisions.json`. The `LocationValidator` singleton lazy-loads this on first use. District and municipality are required and validated on property creation (case-insensitive, each checked independently — the hierarchy is not enforced); parish is optional. `UpdateProperty` does not re-run this validation.

`utils.Slugify` and `LocationValidator.SlugToDistrict/SlugToMunicipality` define the public `/houses/{district}/{municipality}` URLs used by the front end and sitemap — changing slug rules changes indexed URLs.

## Configuration

Config is TOML loaded via Viper. All `ServerConfig` fields are private with public getter methods. Key values:
- `[server].origin` is the CORS allowed origin (comma-separated for multiple); CORS is only enabled when non-empty, with credentials
- `[server].proxy_header` — client-IP header set by a reverse proxy on the same host (see Authentication); empty = direct connections
- `[security].jwt_secret` must be ≥ 32 characters (validated at startup)
- `[database]` — PostgreSQL `host`/`port`/`username`/`password`/`db_name`/`ssl_mode` and pool sizes
- `[database].user_file_path` points to `configs/standardUserList.json` — users there are created on startup (`is_active=true`, not super users) if their email doesn't exist; a missing or invalid file is silently ignored
- `[logging].log_dir` — two loggers, `SERVER` and `DATABASE`, write to `{log_dir}/SERVER.log` / `DATABASE.log` (lumberjack: 25 MB per file, 30 compressed backups, 45 days) and, unless `log_to_console = false`, to stdout. Disabling console output requires a `log_dir` (and the log file must be openable at startup); `log_to_console` accepts only true/false. Under systemd it keeps the app's logs out of the journal — only Fiber's startup banner and fatal startup errors (stderr) still reach it. Levels are `trace|debug|info|warn|error|disabled`; debug/trace log every 4xx and failed-login emails, so production should use info
- `[server].spaFolder` is still parsed but unused — the front end is now a separate SvelteKit SSR server

## Deployment

`scripts/deploy.sh` deploys **both** this repo and the front end on the server: `$PROJECT_ROOT` (default `/opt/immolux`) with `backend/` and `frontend/` clones. It pulls each with `--ff-only`, regenerates Ent code, builds `immo-lux-server` (`go build ./internal`) and the SvelteKit `adapter-node` front end, then restarts the `immolux` and `immolux-frontend` systemd services, health-checking `http://127.0.0.1:8082/v1/api/ping` in between. Flags: `--backend-only`, `--frontend-only`, `--skip-restart`, `--force-install`. The production backend must therefore listen on 8082.

For the backend the server needs Go ≥ the `go` directive in `go.mod` (1.27.1). `deploy.sh` checks it against `REQUIRED_GO_VERSION` — bump both together — and, when the server's Go is older, prints the commands that replace `/usr/local/go` with the go.dev tarball.

PostgreSQL 18 is the target in production as in development. To move the server's cluster to a new major on Debian/Ubuntu: install `postgresql-18`, stop `immolux`, run `pg_upgradecluster <old> main` (it keeps port 5432 for the new cluster, so `config.toml` needs no change), start `immolux` and check it, then `pg_dropcluster <old> main` and purge the old `postgresql-<old>` packages.

For the front end the server needs Node `^22.13 || >=24` (the front end's `.npmrc` sets `engine-strict`, so an older Node fails `npm ci`); `deploy.sh` aborts below that and warns when npm is older than 12, because only npm 12 honours the front end's `package.json` `allowScripts` install-script policy. The recommended setup is Node 26 + npm 12 (NodeSource `setup_26.x`, then `npm install -g npm@12`).

The production `config.toml` lives only on the server; `deploy.sh` never touches it. When new keys are added, add them there **before** deploying. Production should have `[server].proxy_header` (behind the reverse proxy; otherwise every client shares one login rate-limit bucket) and `[logging].log_to_console = false` with an `info` level. The first start after deploying the retention job deletes sessions, auth logs and rate-limit records older than 90 days. Keep `enable_pre_fork = false`: each prefork child re-runs `main`, so it would open the same log files (lumberjack supports one writer process), run its own maintenance job and have its own login limiter.
