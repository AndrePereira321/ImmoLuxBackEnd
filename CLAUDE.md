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
  → Route handler (internal/server/routes/)
  → Repository (internal/database/)
  → Ent ORM (internal/database/ent/client/) [auto-generated, never edit]
```

**Layer map:**
| Directory | Purpose |
|-----------|---------|
| `internal/server/routes/` | HTTP handlers, one file per domain |
| `internal/database/` | Repository implementations (one per entity) |
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
- `ctx.RespondData(data)` / `ctx.RespondError(code, msg, status)` — standardized JSON response
- `ctx.ReadBody(&dto)` — decode + validate request body
- `ctx.DB` / `ctx.Config` / `ctx.Logger` — pre-injected deps

Public vs secured routes are registered via `server.Get()` vs `server.SecuredGet()`. The secured variant blocks unauthenticated requests before reaching the handler.

### API Response Envelope

All responses (success and error) use the same shape:
```json
{ "success": true|false, "data": any|null, "error": { "code": "...", "message": "..." }|null }
```

### Error Handling

Use `server_error` throughout — never return raw errors:
```go
server_error.New("PROPERTY_NOT_FOUND", "property does not exist")
server_error.Wrap("DB_QUERY", "failed to fetch property", err)
server_error.IsServerError(err, "USER_NOT_FOUND") // type check
```

Codes are uppercase snake_case strings; full list is in `internal/server_error/`.

### Transactions

```go
db.WithTransaction(func(ctx context.Context, tx *client.Tx) error {
    // auto-rollback on error, auto-commit on success
})
```

Pass `tx` into repository methods that accept it.

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

Fields marked `.Sensitive()` (e.g., `hash`, `image_data`, `session_token`) are excluded from query results by default.

## Image Handling

Images are stored as BLOBs in the `property_images` table (Sensitive field). On upload:
- Max 10 MB input
- Always converted to JPEG quality 85
- Resized to max 1920×1920 (aspect ratio preserved)
- Supported input: JPEG, PNG, WebP, TIFF, BMP

Served via `GET /v1/api/images/:id` with the stored `content_type` header.

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
