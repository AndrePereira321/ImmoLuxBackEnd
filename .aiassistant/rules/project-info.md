---
apply: always
---

# ImmoLux Back-End Project Guidelines

## Project Overview

**ImmoLux** is a real estate website platform for listing and browsing houses for sale in Portugal (extensible to other countries). This is the backend Go API server that powers the platform.

### User Roles
- **Anonymous Users**: Can search, browse houses, and contact publishers
- **Authenticated Publishers**: Can login and publish house listings (access controlled by superusers)
- **Superusers**: Define and manage which users can publish listings

### Architecture
- RESTful API server built with Go and Fiber v3
- PostgreSQL database with Ent ORM for type-safe queries
- JWT-based authentication with HTTP-only cookies and session management
- Bcrypt password hashing for secure password storage
- Session revocation system with database-backed session tracking
- Structured logging with log rotation
- Configuration via TOML files
- Serves static SPA files from frontend build

## Tech Stack

### Core Technologies
- **Language**: Go 1.25+
- **Web Framework**: Fiber v3 (fiber/v3)
- **ORM**: Ent v0.14+ (entgo.io/ent)
- **Database**: PostgreSQL with lib/pq driver
- **Password Hashing**: bcrypt (golang.org/x/crypto)

### Key Libraries
- **Configuration**: Viper (spf13/viper) for TOML config parsing
- **Logging**: Zerolog (rs/zerolog) with structured logging
- **Log Rotation**: Lumberjack (natefinch/lumberjack.v2)
- **JSON**: goccy/go-json for high-performance JSON encoding/decoding
- **JWT**: golang-jwt/jwt v5 for JSON Web Token generation and validation

### Frontend Integration
- **Frontend Framework**: SvelteKit 2.x (TypeScript SPA)
- **Frontend Location**: `C:\Users\andre\Desktop\Development\immo-lux-front-end`
- **API Prefix**: All API routes prefixed with `/v1/api`
- **SPA Serving**: Backend serves static frontend files with fallback routing

## Go Standards and Conventions

### Code Style
- Follow standard Go conventions (gofmt, golint)
- Use meaningful variable and function names
- Package names should be lowercase, single-word when possible
- Exported identifiers (functions, types, constants) must be PascalCase
- Unexported identifiers must be camelCase

### Error Handling
- **ALWAYS wrap errors** using the `server_error` package
- **NEVER return raw errors** from functions
- **ALWAYS provide context** when wrapping errors
- Use descriptive error codes (e.g., "USER_NOT_FOUND", "DB_CONNECT")

```go
// ✅ GOOD - Wrapped with context
if err != nil {
    return server_error.Wrap("USER_REPOSITORY", "failed querying user", err)
}

// ✅ GOOD - New error with code
if email == "" {
    return server_error.New("USER_VALIDATION", "email is required")
}

// ❌ BAD - Raw error without context
if err != nil {
    return err
}
```

### Logging Requirements
- **ALWAYS log important operations** (user creation, authentication, database operations)
- Use appropriate log levels:
  - `Trace`: Very detailed debugging (SQL queries, low-level operations)
  - `Debug`: Detailed debugging (function entry/exit, intermediate states)
  - `Info`: Important events (server startup, successful operations)
  - `Warn`: Recoverable errors or unexpected situations
  - `Error`: Errors that require attention
  - `Fatal`: Critical errors that force shutdown

```go
// ✅ GOOD - Structured logging with context
rep.db.Logger().Debug(fmt.Sprintf("Creating user: %s", *user.Email))
rep.db.Logger().Error(fmt.Sprintf("Failed to create user [%s]: %s", *user.Email, err.Error()))

// ✅ GOOD - Event-based logging with fields
logger.InfoEvent().Str("email", email).Msg("User logged in successfully")

// ❌ BAD - Using print/println
print("Creating user")  // Never use this
```

### Pointer Usage
- Use pointers for optional fields in DTOs (Data Transfer Objects)
- Use pointers for large structs to avoid copying
- Use value types for small, immutable data
- **ALWAYS check for nil** before dereferencing pointers

```go
// ✅ GOOD - Optional fields as pointers
type UserDTO struct {
    ID        *RecordId  `json:"id"`
    FirstName *string    `json:"firstName"`
    Email     *string    `json:"email"`
}

// ✅ GOOD - Nil check before use
if user.Email != nil && *user.Email != "" {
    // Safe to use
}
```

### Context Usage
- Pass `context.Context` as first parameter in functions
- Use `context.Background()` for top-level operations
- Use `context.WithTimeout()` for operations with timeouts
- Always respect context cancellation

```go
// ✅ GOOD - Context as first parameter
func (rep *UserRepository) CreateUser(ctx context.Context, tx *client.Tx, user *models.UserDTO) error {
    createdUser, err := tx.User.Create().
        SetFirstName(*user.FirstName).
        Save(ctx)
    // ...
}
```

### Function Parameters
- **Avoid functions with more than 3-4 parameters**
- **Use structs for functions with many parameters**
- Group related parameters into configuration structs
- Use descriptive struct field names

```go
// ❌ BAD - Too many parameters
func CreateSession(ctx context.Context, tx *client.Tx, userId int, tokenHash string, rememberMe bool, expiresAt time.Time, ipAddress *string, userAgent *string) error {
    // ...
}

// ✅ GOOD - Parameters grouped in struct
type CreateSessionParams struct {
    UserID     models.RecordId
    TokenHash  string
    RememberMe bool
    ExpiresAt  time.Time
    IpAddress  *string
    UserAgent  *string
}

func CreateSession(ctx context.Context, tx *client.Tx, params CreateSessionParams) error {
    // ...
}
```

### Database Access
- **NEVER access database Client directly from routes or handlers**
- **ALWAYS use repository pattern** for all database operations
- Keep all database logic encapsulated in repositories
- Routes should only call repository methods

```go
// ❌ BAD - Direct client access from route
user, err := ctx.Db().Client().User.Get(context.Background(), int(userId))

// ✅ GOOD - Using repository
user, err := ctx.Db().NewUserRepository().GetUserById(context.Background(), userId)
```

## Project Structure

```
immo-lux-back-end/
├── configs/                    # Configuration files (TOML)
│   ├── config.toml            # Main configuration (gitignored)
│   ├── config.toml.template   # Configuration template
│   ├── standardUserList.json  # Initial users (gitignored)
│   └── standardUserList.json.template
├── db/                        # Database files (if any)
├── logs/                      # Log files (gitignored)
│   ├── SERVER.log
│   ├── DATABASE.log
│   └── DB_UPGRADE.log
├── internal/                  # Private application code
│   ├── config/               # Configuration parsing and structures
│   │   ├── config.go        # Main config types and parser
│   │   ├── app_version.go   # Version parsing and validation
│   │   └── constants.go     # Application constants
│   ├── database/            # Database layer
│   │   ├── database.go      # Database connection and initialization
│   │   ├── user_repository.go  # User data access layer
│   │   └── ent/             # Ent ORM generated and schema code
│   │       ├── schema/      # Ent schema definitions
│   │       │   ├── user.go
│   │       │   ├── userauth.go
│   │       │   └── session.go
│   │       └── client/      # Generated Ent client code
│   ├── logger/              # Logging utilities
│   │   └── logger.go       # Zerolog wrapper with rotation
│   ├── models/              # Domain models and DTOs
│   │   ├── server.go       # API response structures
│   │   ├── auth.go         # Authentication models
│   │   └── database.go     # Database-related types
│   ├── server/              # HTTP server
│   │   ├── server.go       # Server initialization and lifecycle
│   │   ├── routing.go      # Route registration
│   │   ├── memory_fs.go    # In-memory filesystem for SPA
│   │   └── routes/         # Route handlers
│   │       ├── context.go  # Request context wrapper
│   │       ├── auth.go     # Authentication endpoints
│   │       └── system.go   # System endpoints (ping)
│   ├── server_error/        # Error handling package
│   │   └── server_error.go # Custom error types
│   ├── utils/               # Utility functions
│   │   ├── strings.go      # String utilities (email validation)
│   │   └── config.go       # Config utilities (standard users)
│   └── main.go             # Application entry point
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
└── readme.md               # Project documentation
```

## Configuration Management

### TOML Configuration
- **ALL configuration** is in `configs/config.toml`
- **NO environment variables** required for basic setup
- Configuration is read at startup and immutable during runtime
- Use `config.toml.template` as a reference

### Configuration Structure
```toml
[app]
name = "ImmoLux"
version = "1.0.0-alpha"

[server]
host = "localhost"
port = 8082
enable_pre_fork = false
origin = "http://localhost:8080/"  # CORS origins (comma-separated)
spaFolder = "C:\\path\\to\\frontend\\build"

[security]
jwt_secret = "your-super-secret-jwt-key-change-in-production-min-32-chars"
jwt_issuer = "immolux"
cookie_secure = false  # Set to true in production with HTTPS

[logging]
server_log_level = "INFO"      # TRACE, DEBUG, INFO, WARN, ERROR, FATAL
database_log_level = "INFO"
log_dir = "./logs"

[database]
host = "localhost"
port = 5432
username = "immolux"
password = "immolux"
db_name = "immolux"
ssl_mode = "disable"            # disable, require, verify-ca, verify-full
max_idle_connections = 50
max_open_connections = 50
user_file_path = "configs/standardUserList.json"
```

### Accessing Configuration
```go
// ✅ GOOD - Access through type-safe getters
port := serverConfig.HttpServer().Port()
dbURL := serverConfig.Database().ConnectionString()
logLevel := serverConfig.Logging().ServerLogLevel()
jwtSecret := serverConfig.Security().JwtSecret()
cookieSecure := serverConfig.Security().CookieSecure()

// ❌ BAD - Direct field access (fields are private)
port := serverConfig.httpServer.port  // Won't compile
```

## Database Patterns (Ent ORM)

### Schema Definition
- Schemas defined in `internal/database/ent/schema/`
- Run `go generate ./internal/database/ent` to generate client code
- Schemas automatically migrate on application startup

```go
// ✅ GOOD - Well-defined Ent schema
type User struct {
    ent.Schema
}

func (User) Fields() []ent.Field {
    return []ent.Field{
        field.String("email").Unique().NotEmpty().MaxLen(255),
        field.Bool("is_active").Default(true),
        field.Time("created_at").Default(time.Now).Immutable(),
    }
}

func (User) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("auth", UserAuth.Type).Unique(),
        edge.To("sessions", Session.Type),
    }
}

func (User) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("email"),
    }
}
```

### Repository Pattern
- **ALWAYS use repositories** for database access
- Repositories encapsulate database operations
- Repositories should be created through `Database.NewXxxRepository()`

```go
// ✅ GOOD - Using repository
userRepo := db.NewUserRepository()
userId, err := userRepo.GetUserID(email)

// ❌ BAD - Direct client access from routes
user, err := db.Client().User.Query().Where(user.EmailEQ(email)).Only(ctx)
```

### Transaction Handling
- Use `Database.WithTransaction()` for multi-step operations
- Transactions automatically rollback on error
- Transactions automatically commit on success

```go
// ✅ GOOD - Using WithTransaction
err := db.WithTransaction(func(ctx context.Context, tx *client.Tx) error {
    // Create user
    createdUser, err := tx.User.Create().SetEmail(email).Save(ctx)
    if err != nil {
        return err  // Automatic rollback
    }

    // Create auth
    _, err = tx.UserAuth.Create().SetUserID(createdUser.ID).Save(ctx)
    if err != nil {
        return err  // Automatic rollback
    }

    return nil  // Automatic commit
})
```

### Ent Query Patterns
```go
// ✅ GOOD - Proper error handling
user, err := db.client.User.Query().
    Where(user.EmailEQ(email)).
    Only(ctx)
if err != nil {
    if client.IsNotFound(err) {
        return server_error.New("USER_NOT_FOUND", "user not found")
    }
    return server_error.Wrap("USER_REPOSITORY", "failed querying user", err)
}

// ✅ GOOD - Eager loading relationships
user, err := db.client.User.Query().
    Where(user.EmailEQ(email)).
    WithAuth().
    WithSessions().
    Only(ctx)

// ✅ GOOD - Checking existence
exists, err := db.client.User.Query().
    Where(user.EmailEQ(email)).
    Exist(ctx)
```

## API Design Patterns

### Standard Response Format
- **ALL API responses** must use `ServerAPIResponse` structure
- Consistent structure across all endpoints
- Client expects this exact format

```go
type ServerAPIResponse struct {
    Success bool            `json:"success"`
    Data    any             `json:"data"`
    Error   *ServerAPIError `json:"error"`
}

type ServerAPIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### Response Patterns
```go
// ✅ GOOD - Success response
return ctx.RespondData(&LoginResponse{
    IsConnected:  true,
    SessionToken: token,
    UserData:     userDto,
})

// ✅ GOOD - Error response
return ctx.BadRequest("Invalid email")

// ✅ GOOD - Custom error response
return ctx.RespondError(fiber.StatusForbidden, "ACCESS_DENIED", "User is locked")

// ❌ BAD - Direct Fiber response
return ctx.JSON(map[string]string{"error": "not found"})
```

### Route Handler Pattern
```go
// ✅ GOOD - Standard route handler structure
func Login(ctx *RouteContext) error {
    // 1. Parse and validate request
    payload := &LoginPayload{}
    if err := ctx.ReadBody(&payload); err != nil {
        return err
    }

    // 2. Validate input
    if !utils.IsValidEmail(payload.Email) {
        return ctx.BadRequest("Invalid email")
    }

    // 3. Business logic
    user, err := ctx.Db().NewUserRepository().GetUserAuth(payload.Email)
    if err != nil {
        if server_error.IsServerError(err, "USER_NOT_FOUND") {
            return ctx.BadRequest("User not found")
        }
        ctx.Logger().Warn(fmt.Sprintf("Error: %s", err.Error()))
        return err
    }

    // 4. Return response
    return ctx.RespondData(&LoginResponse{
        IsConnected: true,
        UserData:    user,
    })
}
```

### Route Registration

#### Public vs Secured Routes
- **Public routes**: Accessible without authentication (use `Get`, `Post`, `Put`, `Delete`)
- **Secured routes**: Require valid session cookie (use `SecuredGet`, `SecuredPost`, `SecuredPut`, `SecuredDelete`)
- Secured routes automatically return `401 Unauthorized` if session is invalid

```go
// ✅ GOOD - Route registration with public and secured routes
func (s *Server) RegisterRoutes() {
    // Public routes - No authentication required
    s.Get("/ping", routes.Ping)
    s.Post("/login", routes.Login)
    s.Get("/isconnected", routes.IsConnected)
    s.Get("/houses", routes.ListHouses)
    s.Get("/houses/:id", routes.GetHouse)

    // Secured routes - Authentication required
    s.SecuredPost("/logout", routes.Logout)
    s.SecuredPost("/houses", routes.CreateHouse)
    s.SecuredPut("/houses/:id", routes.UpdateHouse)
    s.SecuredDelete("/houses/:id", routes.DeleteHouse)
    s.SecuredGet("/profile", routes.GetProfile)

    // SPA serving (must be last)
    s.serveSPA()
}
```

#### Available Route Methods
- `Get(path, handler)` - Public GET endpoint
- `Post(path, handler)` - Public POST endpoint
- `Put(path, handler)` - Public PUT endpoint
- `Delete(path, handler)` - Public DELETE endpoint
- `SecuredGet(path, handler)` - Authenticated GET endpoint
- `SecuredPost(path, handler)` - Authenticated POST endpoint
- `SecuredPut(path, handler)` - Authenticated PUT endpoint
- `SecuredDelete(path, handler)` - Authenticated DELETE endpoint

### API Route Conventions
- Use RESTful conventions
- Routes automatically prefixed with `/v1/api`
- Use HTTP methods appropriately (GET, POST, PUT, DELETE)
- Use plural nouns for resource collections (`/houses`, `/users`)
- Use path parameters for specific resources (`/houses/:id`)

```go
// Examples:
// GET    /v1/api/houses           - List all houses
// GET    /v1/api/houses/:id       - Get specific house
// POST   /v1/api/houses           - Create new house
// PUT    /v1/api/houses/:id       - Update house
// DELETE /v1/api/houses/:id       - Delete house
// POST   /v1/api/login            - Login (action, not resource)
// POST   /v1/api/logout           - Logout (action, not resource)
```

## RouteContext Pattern

### Using RouteContext
- **ALWAYS use RouteContext** for all route handlers
- Provides access to logger, database, and user context
- Encapsulates common response patterns

```go
// ✅ GOOD - Using RouteContext helpers
func MyRoute(ctx *RouteContext) error {
    // Access logger
    ctx.Logger().Info("Processing request")

    // Access database
    userRepo := ctx.Db().NewUserRepository()

    // Read request body
    payload := &MyPayload{}
    if err := ctx.ReadBody(&payload); err != nil {
        return err
    }

    // Return success
    return ctx.RespondData(result)

    // Return error
    return ctx.BadRequest("Invalid input")
}
```

## Error Handling Standards

### ServerError Structure
```go
type ServerError struct {
    Code    string  // Error code (e.g., "USER_NOT_FOUND")
    Message string  // Human-readable message
    Cause   error   // Underlying error (can be nil)
}
```

### Creating Errors
```go
// ✅ GOOD - New error without cause
if email == "" {
    return server_error.New("VALIDATION_ERROR", "email is required")
}

// ✅ GOOD - Wrapping existing error
if err != nil {
    return server_error.Wrap("DB_ERROR", "failed to query user", err)
}
```

### Checking Error Types
```go
// ✅ GOOD - Checking for specific error
if server_error.IsServerError(err, "USER_NOT_FOUND") {
    return ctx.BadRequest("User not found")
}

// ✅ GOOD - Checking if it's any ServerError
var serverErr *server_error.ServerError
if errors.As(err, &serverErr) {
    // Handle server error
}
```

### Error Code Conventions
- Use UPPERCASE_SNAKE_CASE for error codes
- Use descriptive, specific codes
- Group related errors with common prefixes

```go
// ✅ GOOD - Descriptive error codes
"USER_NOT_FOUND"
"USER_ALREADY_EXISTS"
"USER_VALIDATION"
"DB_CONNECT"
"DB_MIGRATE"
"DB_TX"
"CONFIG_PARSER"
"AUTH_INVALID_PASSWORD"
"AUTH_USER_LOCKED"
"SESSION_EXPIRED"

// ❌ BAD - Vague error codes
"ERROR"
"FAILED"
"BAD"
```

## Security Best Practices

### Authentication System

**Complete documentation**: See `auth-flow.md` in the project root for detailed authentication flow documentation.

The ImmoLux authentication system implements:
- **JWT tokens** stored in HTTP-only cookies for XSS protection
- **Session management** in database for server-side revocation
- **Remember Me** functionality (1 hour vs 30 days sessions)
- **Failed login tracking** with automatic account locking after 5 attempts
- **Specific error codes** for different authentication failures

**Key authentication error codes:**
- `COOKIE_MISSING` - Session cookie not found
- `JWT_EXPIRED` - Session token has expired
- `JWT_INVALID` - Session token signature is invalid
- `SESSION_NOT_FOUND` - Session ID from JWT not in database
- `SESSION_INVALID` - Session was revoked or expired
- `INVALID_CREDENTIALS` - Wrong email or password
- `ACCOUNT_LOCKED` - Too many failed login attempts

**Authentication endpoints:**
- `POST /api/login` - Login with email, password, rememberMe
- `GET /api/isconnected` - Check authentication status
- `POST /api/logout` - Logout and invalidate session (secured route)

**Files involved:**
- `internal/server/routes/auth.go` - Login, IsConnected, Logout handlers
- `internal/server/routes/context.go` - Authentication extraction and validation
- `internal/utils/jwt.go` - JWT generation and validation
- `internal/utils/session.go` - Session token hashing and expiration
- `internal/database/session_repository.go` - Session CRUD operations
- `internal/database/user_repository.go` - User authentication operations
- `internal/config/constants.go` - SessionCookieName constant

### Password Handling
```go
// ✅ GOOD - Using bcrypt
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// ✅ GOOD - Validating password
err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(providedPassword))
if err != nil {
    // Invalid password
}
```

### Sensitive Data in Ent Schemas
```go
// ✅ GOOD - Mark sensitive fields
field.String("hash").NotEmpty().Sensitive(),
field.String("session_token").Unique().NotEmpty().Sensitive(),

// ✅ GOOD - Exclude from JSON serialization
type UserAuthDTO struct {
    Hash *string `json:"-"`  // Never sent to client
}
```

### Input Validation
```go
// ✅ GOOD - Validate all inputs
if !utils.IsValidEmail(email) {
    return server_error.New("VALIDATION_ERROR", "invalid email")
}

if len(password) < models.PasswordMinLength {
    return server_error.New("VALIDATION_ERROR",
        fmt.Sprintf("password must be at least %d characters", models.PasswordMinLength))
}
```

### CORS Configuration
- Configure CORS origins in config.toml
- Supports multiple origins (comma-separated)
- Credentials support enabled

```toml
[server]
origin = "http://localhost:8080,https://immolux.com"
```

## Logging Standards

### Logger Creation
```go
// ✅ GOOD - Create named loggers
logger, err := logger.New("SERVER", level, logDir)
logger, err := logger.New("DATABASE", level, logDir)
logger, err := logger.New("AUTH", level, logDir)
```

### Logging Patterns
```go
// ✅ GOOD - Simple message
logger.Info("Server started successfully")
logger.Debug("Processing user authentication")
logger.Warn("Database connection pool exhausted")
logger.Error("Failed to connect to database")

// ✅ GOOD - Structured logging with events
logger.InfoEvent().
    Str("email", email).
    Int("user_id", userId).
    Msg("User logged in")

logger.ErrorEvent().
    Err(err).
    Str("operation", "CreateUser").
    Msg("Failed to create user")

// ✅ GOOD - Context in messages
logger.Debug(fmt.Sprintf("Creating user: %s", email))
logger.Error(fmt.Sprintf("Failed to create user [%s]: %s", email, err.Error()))

// ❌ BAD - Using standard library log
log.Println("Something happened")  // Never use this
```

### Log Levels Guide
```go
// TRACE - Very detailed debugging
logger.Trace("Entering function CreateUser")
logger.Trace("SQL: SELECT * FROM users WHERE email = ?")

// DEBUG - Detailed debugging
logger.Debug(fmt.Sprintf("Creating user: %s", email))
logger.Debug("Connection pool status: 10/50 connections used")

// INFO - Important events
logger.Info("Server started on localhost:8082")
logger.Info("Database schema migrated successfully")

// WARN - Recoverable issues
logger.Warn("Retrying database connection")
logger.Warn("Session expired but user is still active")

// ERROR - Errors requiring attention
logger.Error("Failed to send email notification")
logger.ErrorEvent().Err(err).Msg("Database query failed")

// FATAL - Critical errors forcing shutdown
logger.Fatal("Cannot connect to database after 5 retries")
```

## DTOs (Data Transfer Objects)

### DTO Patterns
- Use pointers for all fields to distinguish between "not set" and "zero value"
- Use JSON tags for API serialization
- Use `json:"-"` to exclude sensitive fields

```go
// ✅ GOOD - DTO with pointer fields
type UserDTO struct {
    ID        *RecordId  `json:"id"`
    FirstName *string    `json:"firstName"`
    LastName  *string    `json:"lastName"`
    Email     *string    `json:"email"`
    IsActive  *bool      `json:"isActive"`
    CreatedAt *time.Time `json:"createdAt"`
    UpdatedAt *time.Time `json:"updatedAt"`
}

// ✅ GOOD - Sensitive field excluded
type UserAuthDTO struct {
    ID     *RecordId `json:"id"`
    Hash   *string   `json:"-"`  // Never sent to client
    IsLocked *bool   `json:"isLocked"`
}
```

### Converting Between Ent and DTOs
```go
// ✅ GOOD - Ent entity to DTO
userId := models.RecordId(entUser.ID)
userDTO := &models.UserDTO{
    ID:        &userId,
    FirstName: &entUser.FirstName,
    LastName:  &entUser.LastName,
    Email:     &entUser.Email,
    IsActive:  &entUser.IsActive,
    CreatedAt: &entUser.CreatedAt,
    UpdatedAt: &entUser.UpdatedAt,
}

// ✅ GOOD - DTO to Ent creation
createdUser, err := tx.User.Create().
    SetFirstName(*userDTO.FirstName).
    SetLastName(*userDTO.LastName).
    SetEmail(*userDTO.Email).
    SetIsActive(*userDTO.IsActive).
    Save(ctx)
```

## Testing Guidelines

### Test File Naming
- Test files must end with `_test.go`
- Place test files in the same package as the code being tested
- Use table-driven tests for multiple scenarios

```go
// ✅ GOOD - Table-driven test
func TestIsValidEmail(t *testing.T) {
    tests := []struct {
        name  string
        email string
        want  bool
    }{
        {"valid email", "user@example.com", true},
        {"invalid email", "not-an-email", false},
        {"empty email", "", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsValidEmail(tt.email); got != tt.want {
                t.Errorf("IsValidEmail() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Organization
```go
// ✅ GOOD - Test structure
func TestFunctionName(t *testing.T) {
    // Setup
    // ...

    // Execute
    result, err := FunctionUnderTest(input)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Code Comments

### When to Comment
- **Do NOT over-comment** - avoid excessive or obvious comments
- **ONLY comment when necessary**: Weird code, workarounds, or very complex algorithms
- **NEVER add obvious comments** that just repeat what the code does
- **Remove inline comments** that explain simple operations
- Only add comments when something is:
  - Complex algorithm or business logic that isn't immediately clear
  - Non-obvious workarounds or edge cases
  - Public API functions and types (godoc)
  - Critical security or performance considerations
  - Unexpected behavior that differs from normal conventions

### Godoc Comments
```go
// ✅ GOOD - Godoc for exported functions
// New creates a new ServerError with the given code and message.
// The error has no underlying cause.
func New(code, message string) *ServerError {
    return newServerError(code, message, nil)
}

// ✅ GOOD - Godoc for types
// ServerError represents a structured error with a code, message, and optional cause.
// It implements the error interface and supports error wrapping.
type ServerError struct {
    Code    string
    Message string
    Cause   error
}

// ❌ BAD - Obvious comments
// Create user
func CreateUser() {}  // Comment adds no value
```

### Examples
```go
// ✅ GOOD - Explaining non-obvious behavior
// Backend expects 200 status even for validation errors.
// Check the success flag to determine actual result.
if !response.Success {
    return response.Error
}

// ✅ GOOD - Explaining complex logic
// Calculate session expiration: 24 hours for remember-me,
// 1 hour for regular sessions, adjusted for user's timezone
expiresAt := calculateExpiration(rememberMe, userTimezone)

// ❌ BAD - Obvious comments
// Get user from database
user, err := repo.GetUser(id)

// ❌ BAD - Commented-out code
// userId, err := createUser(email)
// if err != nil {
//     return err
// }
```

## Development Workflow

### Required Tools
- Go 1.25+
- PostgreSQL 12+
- Air (for hot reload, optional)
- golangci-lint (for linting, optional)

### Common Commands
```bash
# Install dependencies
go mod tidy

# Generate Ent client code (after schema changes)
go generate ./internal/database/ent

# Build the application
go build -o immo-lux-server ./internal

# Run the application
./immo-lux-server configs/config.toml

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Lint code (if golangci-lint installed)
golangci-lint run
```

### Pre-commit Checklist
1. Run `go fmt ./...` - format all code
2. Run `go test ./...` - ensure all tests pass
3. Run `go generate ./internal/database/ent` - if schemas changed
4. Check `go mod tidy` - ensure dependencies are clean
5. Verify configuration template is up-to-date
6. Check that no sensitive data is in commits (passwords, tokens)

### Hot Reload with Air
Create `.air.toml`:
```toml
[build]
  cmd = "go build -o ./tmp/main ./internal"
  bin = "./tmp/main"
  args_bin = ["configs/config.toml"]
  include_ext = ["go", "toml"]
  exclude_dir = ["tmp", "vendor", "logs", "db"]

[log]
  main_only = true
```

Run: `air`

## File Organization Best Practices

### Package Structure
```go
// ✅ GOOD - Clear package organization
internal/
├── config/       # Configuration parsing
├── database/     # Database layer (repositories, Ent)
├── logger/       # Logging utilities
├── models/       # Domain models and DTOs
├── server/       # HTTP server and routing
├── server_error/ # Error handling
└── utils/        # Shared utilities
```

### Import Organization
```go
// ✅ GOOD - Imports in groups
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // External packages
    "github.com/gofiber/fiber/v3"
    "golang.org/x/crypto/bcrypt"

    // Internal packages
    "immo-lux/internal/config"
    "immo-lux/internal/database"
    "immo-lux/internal/server_error"
)
```

### File Naming
- Use lowercase with underscores: `user_repository.go`, `server_error.go`
- Test files: `user_repository_test.go`
- Keep related functionality in the same file
- Split large files by domain, not by functionality type

## Performance Considerations

### Database Connection Pool
```toml
[database]
max_idle_connections = 50  # Adjust based on load
max_open_connections = 50  # Adjust based on load
```

### In-Memory SPA Serving
- Frontend files loaded into memory at startup
- Faster serving than disk I/O
- Requires server restart when frontend rebuilds

### JSON Performance
- Using `goccy/go-json` for faster JSON operations
- Configured in Fiber app initialization

## Production Recommendations

### Security
- Change database SSL mode to `require` or `verify-full`
- Use strong passwords in configuration
- Consider secrets management instead of config file passwords
- Enable HTTPS with TLS certificates
- Implement rate limiting for authentication endpoints
- Add request timeouts
- Implement proper session expiration

### Database
- Use PostgreSQL 14+ for best performance
- Enable connection pooling with appropriate limits
- Set up database backups
- Monitor slow queries
- Use prepared statements (Ent handles this)
- Create indexes for frequently queried fields

### Logging
- Set production log level to `INFO` or `WARN`
- Ensure log rotation is configured (max 10MB, 30 backups, 45 days)
- Monitor log files for errors
- Consider centralized logging (e.g., ELK stack)

### Monitoring
- Monitor database connection pool usage
- Track API response times
- Monitor error rates
- Set up health checks (`/v1/api/ping`)
- Monitor memory usage and goroutine counts

### Deployment
- Build with optimizations: `go build -ldflags="-s -w"`
- Use process manager (systemd, supervisord)
- Configure graceful shutdown
- Set appropriate timeouts
- Use reverse proxy (nginx, Caddy) for HTTPS termination

## Common Patterns and Utilities

### RecordId Type
```go
// Custom type for database record IDs
type RecordId int64

const InvalidRecordId = RecordId(-1)
const UnknownRecordId = RecordId(-2)

// ✅ GOOD - Check validity
if userId.IsValid() {
    // Use userId
}
```

### Email Validation
```go
// ✅ GOOD - Using utility function
if !utils.IsValidEmail(email) {
    return server_error.New("VALIDATION_ERROR", "invalid email")
}
```

### Standard Users Initialization
- Standard users loaded from `configs/standardUserList.json`
- Automatically created on first startup if not exists
- Useful for initial admin accounts

```json
[
  {
    "firstName": "Admin",
    "lastName": "User",
    "email": "admin@immolux.com",
    "password": "secure-password"
  }
]
```

## Graceful Shutdown

```go
// ✅ GOOD - Proper shutdown handling
signalCh := make(chan os.Signal, 1)
signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

select {
case <-signalCh:
    shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()

    if err := httpServer.Shutdown(shutdownCtx); err != nil {
        return server_error.Wrap("SHUTDOWN", "failed to shutdown gracefully", err)
    }
}
```

## Frontend Integration

### Frontend Project
- **Location**: `C:\Users\andre\Desktop\Development\immo-lux-front-end`
- **Framework**: SvelteKit 2.x with Svelte 5.x (TypeScript SPA)
- **Styling**: TailwindCSS 4.x
- **Build Tool**: Vite 7.x
- **Adapter**: @sveltejs/adapter-static (SPA mode)
- **HTTP Client**: Axios 1.x
- **i18n**: svelte-i18n (Portuguese, English, French)
- **Icons**: FontAwesome (Solid, Regular, Brands)

### Frontend Guidelines
The frontend has its own comprehensive guidelines at:
`C:\Users\andre\Desktop\Development\immo-lux-front-end\.aiassistant\rules\project-info.md`

Key frontend principles:
- **TypeScript strict mode** - Everything must be typed, no `any` types
- **Svelte 5 runes** - Uses modern `$state`, `$derived`, `$effect`, `$props`
- **Internationalization** - All user-facing text must be i18n-ready
- **TailwindCSS-first** - Utility classes for all styling
- **Component-driven** - Small, focused components with single responsibility

### API Response Format
The frontend expects this exact format for ALL API responses:
```typescript
interface ServerAPIResponse<T> {
  success: boolean;
  data: T;
  error: ServerAPIError | null;
}

interface ServerAPIError {
  code?: string;
  message: string;
}
```

**Backend Implementation:**
```go
// ✅ GOOD - Success response
return ctx.RespondData(&LoginResponse{
    IsConnected:  true,
    SessionToken: token,
    UserData:     userDto,
})
// Produces: { success: true, data: {...}, error: null }

// ✅ GOOD - Error response
return ctx.BadRequest("Invalid email format")
// Produces: { success: false, data: null, error: { code: "BAD_REQUEST", message: "Invalid email format" } }

// ❌ BAD - Non-standard response
return ctx.JSON(fiber.Map{"error": "something went wrong"})
```

### API Routes and Prefixing
- **Backend API Prefix**: `/v1/api`
- **Frontend API Client**: Automatically adds `/v1/api/` prefix to all routes

**Example Flow:**
```typescript
// Frontend code
const response = await apiClient.get('/houses');
// Actual request: GET http://localhost:8082/v1/api/houses

const response = await apiClient.post('/login', credentials);
// Actual request: POST http://localhost:8082/v1/api/login
```

**Backend route registration:**
```go
func (s *Server) RegisterRoutes() {
    // These automatically become /v1/api/houses, /v1/api/login, etc.
    s.Get("/houses", routes.ListHouses)
    s.Post("/login", routes.Login)
    s.Get("/ping", routes.Ping)
}
```

### CORS Configuration
The backend serves CORS headers for frontend domain access:

```toml
[server]
# In config.toml - comma-separated origins for multiple environments
origin = "http://localhost:8080,https://immolux.com"
```

**CORS Settings:**
- `AllowCredentials: true` - Enables session cookies
- `AllowOrigins`: From config.toml
- `AllowHeaders`: Origin, Content-Type, Accept
- `AllowMethods`: GET, POST, PUT, DELETE, OPTIONS

### SPA Serving
The backend serves the frontend static files directly:

```toml
[server]
# Path to frontend build output
spaFolder = "C:\\Users\\andre\\Desktop\\Development\\immo-lux-front-end\\build"
```

**How it works:**
1. Frontend builds to `build/` directory using `npm run build`
2. Backend loads all files into memory at startup (memory_fs.go)
3. Backend serves SPA with fallback routing:
   - API routes (`/v1/api/*`) → Go route handlers
   - Static files (`/assets/*`, `/favicon.ico`) → Served from memory
   - All other routes → `index.html` (SPA client-side routing)

**Deployment workflow:**
```bash
# 1. Build frontend
cd C:\Users\andre\Desktop\Development\immo-lux-front-end
npm run build

# 2. Backend automatically serves from build folder
# No separate web server needed!
```

### Frontend API Client Integration
The frontend uses a centralized ApiClient class:

**Frontend ApiClient (src/lib/api/api-client.ts):**
```typescript
class ApiClient {
  private baseURL: string;
  private axiosInstance: AxiosInstance;

  constructor() {
    // Uses VITE_SERVER_URL env var or falls back to current origin
    this.baseURL = import.meta.env.VITE_SERVER_URL || window.location.origin;

    this.axiosInstance = axios.create({
      baseURL: `${this.baseURL}/v1/api`,  // Auto-prefix
      withCredentials: true,               // Send cookies
      validateStatus: () => true           // Accept all status codes
    });
  }

  async get<T>(url: string): Promise<AxiosResponse<ServerAPIResponse<T>>> {
    return this.axiosInstance.get(url);
  }

  async post<T>(url: string, data?: any): Promise<AxiosResponse<ServerAPIResponse<T>>> {
    return this.axiosInstance.post(url, data);
  }
}
```

**Backend must always return ServerAPIResponse:**
```go
// ✅ GOOD - Consistent response structure
type LoginResponse struct {
    IsConnected  bool            `json:"isConnected"`
    SessionToken string          `json:"sessionToken"`
    UserData     *models.UserDTO `json:"userData"`
}

func Login(ctx *RouteContext) error {
    // ... authentication logic ...
    return ctx.RespondData(&LoginResponse{
        IsConnected:  true,
        SessionToken: sessionToken,
        UserData:     userDto,
    })
}
```

### Frontend Type Definitions
The frontend has TypeScript interfaces that should match backend DTOs:

**Frontend (src/lib/types/api.ts):**
```typescript
export interface UserDTO {
  id: number;
  firstName: string;
  lastName: string;
  email: string;
  isActive: boolean;
  createdAt: Date;
  updatedAt: Date;
}

export interface House {
  id: number;
  title: string;
  description: string;
  price: number;
  location: string;
  images: string[];
  publisherId: number;
  createdAt: Date;
  updatedAt: Date;
}
```

**Backend (internal/models/):**
```go
// Must use JSON tags matching frontend camelCase
type UserDTO struct {
    ID        *RecordId  `json:"id"`
    FirstName *string    `json:"firstName"`  // camelCase!
    LastName  *string    `json:"lastName"`   // camelCase!
    Email     *string    `json:"email"`
    IsActive  *bool      `json:"isActive"`   // camelCase!
    CreatedAt *time.Time `json:"createdAt"`  // camelCase!
    UpdatedAt *time.Time `json:"updatedAt"`  // camelCase!
}
```

**Important:**
- Backend uses snake_case in database (Ent schemas)
- Backend uses camelCase in JSON responses (json tags)
- Frontend uses camelCase everywhere (TypeScript)

### Internationalization Support
The frontend supports 3 languages - backend should be aware for:
- Error messages (consider localizing)
- Date/time formatting (consider timezone support)
- Currency formatting (EUR for Portugal)

**Languages:**
- Portuguese (pt) - Default/fallback
- English (en)
- French (fr)

**Frontend usage:**
```typescript
// Frontend displays localized messages
{$t('errors.login.invalid_credentials')}
```

**Backend consideration:**
```go
// Return error codes, let frontend localize
return ctx.RespondError(fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")

// Frontend maps error codes to translations:
// INVALID_CREDENTIALS → pt: "Email ou senha inválidos"
//                     → en: "Invalid email or password"
//                     → fr: "Email ou mot de passe invalide"
```

### Development Setup
**Running both frontend and backend:**

```bash
# Terminal 1 - Backend (serves API)
cd C:\Users\andre\Desktop\Development\immo-lux-back-end
go run ./internal configs/config.toml

# Terminal 2 - Frontend (dev server with HMR)
cd C:\Users\andre\Desktop\Development\immo-lux-front-end
npm run dev
```

**Development URLs:**
- Frontend dev server: `http://localhost:8080` (Vite)
- Backend API server: `http://localhost:8082`
- Frontend calls backend via CORS

**Frontend .env configuration:**
```bash
# In C:\Users\andre\Desktop\Development\immo-lux-front-end\.env
VITE_SERVER_URL=http://localhost:8082
```

### Production Deployment
**Single-server deployment:**

```bash
# 1. Build frontend
cd C:\Users\andre\Desktop\Development\immo-lux-front-end
npm run build
# Output: build/ directory

# 2. Configure backend to serve frontend
# In config.toml:
[server]
spaFolder = "C:\\path\\to\\build"

# 3. Run backend
cd C:\Users\andre\Desktop\Development\immo-lux-back-end
go build -o immo-lux-server ./internal
./immo-lux-server configs/config.toml

# Backend now serves both API and frontend!
# Access at: http://localhost:8082
```

**Multi-server deployment:**
- Frontend served by nginx/Caddy as static files
- Backend as separate API server
- Update CORS origins in backend config.toml

### Common Integration Patterns

#### Authentication Flow
```go
// Backend: Login endpoint
func Login(ctx *RouteContext) error {
    payload := &LoginPayload{}
    if err := ctx.ReadBody(&payload); err != nil {
        return err
    }

    // Validate credentials
    user, err := authenticateUser(payload.Email, payload.Password)
    if err != nil {
        return ctx.RespondError(fiber.StatusUnauthorized,
            "INVALID_CREDENTIALS", "Invalid email or password")
    }

    // Create session
    sessionToken := createSession(user.ID)

    return ctx.RespondData(&LoginResponse{
        IsConnected:  true,
        SessionToken: sessionToken,
        UserData:     user,
    })
}
```

```typescript
// Frontend: Login component
const login = async (email: string, password: string) => {
  const response = await apiClient.post('/login', { email, password });
  const result: ServerAPIResponse<LoginResponse> = response.data;

  if (result.success) {
    localStorage.setItem('sessionToken', result.data.sessionToken);
    userData.set(result.data.userData);
    goto('/panel');
  } else {
    errorMessage = $t(`errors.${result.error?.code}`) || result.error?.message;
  }
};
```

#### List/Search Pattern
```go
// Backend: List houses endpoint
type HouseListResponse struct {
    Houses []models.HouseDTO `json:"houses"`
    Total  int               `json:"total"`
}

func ListHouses(ctx *RouteContext) error {
    // Parse query params
    search := ctx.Ctx().Query("search")
    limit := ctx.Ctx().QueryInt("limit", 20)
    offset := ctx.Ctx().QueryInt("offset", 0)

    houses, total, err := ctx.Db().NewHouseRepository().List(search, limit, offset)
    if err != nil {
        return err
    }

    return ctx.RespondData(&HouseListResponse{
        Houses: houses,
        Total:  total,
    })
}
```

```typescript
// Frontend: List houses page
let houses = $state<House[]>([]);
let total = $state(0);
let loading = $state(false);

const loadHouses = async (search: string, limit: number, offset: number) => {
  loading = true;
  const response = await apiClient.get<HouseListResponse>(
    `/houses?search=${encodeURIComponent(search)}&limit=${limit}&offset=${offset}`
  );

  if (response.data.success) {
    houses = response.data.data.houses;
    total = response.data.data.total;
  }
  loading = false;
};
```

### Error Handling Best Practices

**Backend:**
```go
// Use specific error codes that frontend can translate
if !utils.IsValidEmail(email) {
    return ctx.RespondError(fiber.StatusBadRequest,
        "INVALID_EMAIL", "Email format is invalid")
}

if user == nil {
    return ctx.RespondError(fiber.StatusNotFound,
        "USER_NOT_FOUND", "User does not exist")
}

if user.IsLocked {
    return ctx.RespondError(fiber.StatusForbidden,
        "USER_LOCKED", "User account is locked")
}
```

**Frontend:**
```typescript
// Handle different error codes appropriately
const handleError = (error: ServerAPIError) => {
  switch (error.code) {
    case 'INVALID_EMAIL':
    case 'USER_NOT_FOUND':
      // Show inline validation error
      fieldError = $t(`validation.${error.code}`);
      break;

    case 'USER_LOCKED':
      // Show modal with contact support
      showLockedAccountModal();
      break;

    default:
      // Generic error message
      toastError($t('errors.generic'));
  }
};
```

### Frontend Build Output
The frontend builds to a `build/` directory with this structure:

```
build/
├── index.html          # SPA entry point (fallback for all routes)
├── _app/              # SvelteKit app chunks
│   ├── immutable/    # Hashed, cacheable assets
│   │   ├── chunks/   # JS chunks
│   │   ├── entry/    # Entry points
│   │   └── assets/   # CSS, fonts
│   └── version.json  # Build version
├── favicon.ico
└── [other static files]
```

**Backend serves with:**
- `index.html` → All non-API, non-static routes (SPA routing)
- `/assets/*` → Static assets with proper MIME types
- `/v1/api/*` → API endpoints (not from filesystem)

---

**Remember**: This is a Go backend with strict error handling, comprehensive logging, and type-safe database access through Ent ORM. Every error must be wrapped, every important operation must be logged, and all database access should go through repositories. The backend must always return the `ServerAPIResponse` format that the TypeScript frontend expects.
