# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o immo-lux ./internal/main.go

# Run (config path is required as first arg)
./immo-lux ./configs/config.toml

# Tests
go test ./...
go test ./internal/config/...     # Single package

# Dependencies
go mod tidy

# Regenerate Ent ORM code (run after any schema change)
go generate ./internal/database/ent
```

## Architecture

**Entry point:** `internal/main.go` — reads config path from `os.Args[1]`, initializes DB (runs migrations + seeds users), registers routes, starts Fiber server with graceful shutdown on `SIGINT`/`SIGTERM`.

**Request flow:**
```
Fiber Router (/v1/api prefix)
  → handleRoute() [checks secured flag → calls extractUserContext() → validates JWT + DB session]
  → RouteContext [pre-populated with logger, db, config, auth info]
  → Route handler (internal/server/routes/) [parse → call store operation → respond]
  → Domain store (internal/database/) [owner-scoped operations that own their transactions]
  → Ent ORM (internal/database/ent/client/) [auto-generated, never edit]
```

Handlers never see `*client.Tx` or any other ent type — the ORM stays behind the
`internal/database` seam. Errors returned by handlers are mapped to HTTP status
codes in exactly one place (`handleServerError` in `internal/server/server.go`).

**Layer map:**
| Directory | Purpose |
|-----------|---------|
| `internal/server/routes/` | HTTP handlers, one file per domain |
| `internal/database/` | Domain stores (one per entity) + `AuthOperations` — owner-scoped, transaction-owning |
| `internal/database/ent/schema/` | Ent schema definitions — edit these |
| `internal/database/ent/client/` | Auto-generated Ent code — never edit |
| `internal/models/` | All DTOs and request/response structs |
| `internal/config/` | TOML config via Viper, struct-based with getter methods |
| `internal/server_error/` | `ServerError` type (Code + Message + Cause) |
| `internal/utils/` | JWT, image processing, location validation, session hashing |
| `internal/logger/` | zerolog wrapper with file rotation (lumberjack) |

## Key Patterns

### RouteContext

Every handler receives `*routes.RouteContext` instead of `*fiber.Ctx`. It provides:
- `ctx.IsAuthenticated()` / `ctx.GetAuthError()` — auth state
- `ctx.RespondData(data)` — standardized success envelope
- `ctx.ReadBody(&dto)` — decode request body (returns a kinded 400 error on bad JSON)
- `ctx.ParseIdParam("id")` / `ctx.RequireUserId()` — return kinded errors, never respond themselves
- `ctx.RequestContext()` — request-scoped `context.Context`; pass it to every store call
- `ctx.Db()` / `ctx.ServerConfig()` / `ctx.Logger()` — pre-injected deps

**Handlers respond with data or return an error — never both.** Error-to-HTTP
mapping happens centrally; handlers must not pick status codes.

Public vs secured routes are registered via `server.Get()` vs `server.SecuredGet()`. The secured variant blocks unauthenticated requests before reaching the handler.

### API Response Envelope

All responses (success and error) use the same shape:
```json
{ "success": true|false, "data": any|null, "error": { "code": "...", "message": "..." }|null }
```

### Error Handling

`ServerError` carries a **Kind** that decides the HTTP status; the mapping lives
in exactly one place (`httpStatus` in `internal/server/server.go`). Construct
errors with the kinded constructors and just `return err` from handlers:

```go
server_error.NotFound("PROPERTY_NOT_FOUND", "property not found")     // → 404
server_error.Forbidden("ACCESS_DENIED", "not your property")          // → 403
server_error.Invalid("PROPERTY_VALIDATION", "title is required")      // → 400
server_error.Unauthorized("INVALID_CREDENTIALS", "bad credentials")   // → 401
server_error.RateLimited("RATE_LIMIT_EXCEEDED", "try again later")    // → 429
server_error.New("...", "...") / server_error.Wrap("...", "...", err) // → 500 (internal)
server_error.IsServerError(err, "USER_NOT_FOUND")                     // code check
```

Codes are uppercase snake_case strings. The central adapter also logs: 5xx at
Error level, 4xx at Debug — handlers don't log their own failures.

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

All DTO fields are pointers (`*string`, `*int64`, etc.) so unset fields serialize as `null`. `RecordId` is `int64`; `InvalidRecordId = -1`, `UnknownRecordId = -2`.

## Authentication

JWT is stored in the `session_token` HttpOnly cookie. The token itself is **not** stored in the database — only its SHA-256 hash is stored in the Session table. Both must be valid for a request to be authenticated:
1. JWT signature, expiration, issuer check (via `utils.ValidateJWT`)
2. Session must exist in DB with `is_active=true` and `expires_at > now`

`rememberMe=true` → 30-day JWT + session; `rememberMe=false` → 1-hour JWT + session.

Rate limiting on login: 5 attempts per 15-minute window per `(email, ip_address, action)`. 30-minute block on limit exceeded.

## Ent ORM

Schema definitions live in `internal/database/ent/schema/`. After any change:

```bash
go generate ./internal/database/ent
```

This runs `entgo.io/ent/cmd/ent generate` and regenerates everything under `internal/database/ent/client/`. Do not edit files in `client/` directly.

Schema migrations run automatically at startup via `entClient.Schema.Create(ctx)`.

Fields marked `.Sensitive()` (e.g., `hash`, `image_data`, `session_token`) are hidden from `String()` and JSON marshaling only — **they are still selected by queries**. Image metadata queries therefore project through `imageMetadataColumns` (`property_image_repository.go`) so blobs never leave the database; only `ImagePayload` reads `image_data`.

## Image Handling

Images are stored as BLOBs in the `property_images` table (Sensitive field). On upload:
- Max 10 MB input
- Always converted to JPEG quality 85
- Resized to max 1920×1920 (aspect ratio preserved)
- Supported input: JPEG, PNG, WebP, TIFF, BMP

Served via `GET /v1/api/images/:id` with the stored `content_type` header.

The image store has two faces: metadata queries (gallery listing, ownership
resolution, display order) project only metadata columns, while `ImagePayload`
is the single query that reads the blob. Keep new queries on the right side of
that split.

## Location Validation

Portugal's administrative divisions (districts → municipalities → parishes) are embedded as a JSON file at `internal/utils/data/portugal-admin-divisions.json`. The `LocationValidator` singleton lazy-loads this on first use. District and municipality are required and validated on property creation; parish is optional.

## Configuration

Config is TOML loaded via Viper. All `ServerConfig` fields are private with public getter methods. Key values:
- `[security].jwt_secret` must be ≥ 32 characters (validated at startup)
- `[database].user_file_path` points to `configs/standardUserList.json` — users there are auto-created as `is_super_user=true` on startup
- `[server].origin` is the CORS allowed origin (comma-separated for multiple)
- `[server].spaFolder` optional path to serve the frontend SPA static files

## Deployment

`scripts/deploy.sh` handles full deployment: validates toolchain, pulls git, regenerates Ent code, builds backend and frontend, restarts the systemd service. Default project root is `/opt/immolux`.
