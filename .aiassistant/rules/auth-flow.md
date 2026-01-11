---
apply: always
---

# ImmoLux Authentication Flow Documentation

This document describes the complete authentication system implementation in the ImmoLux backend, intended to guide
frontend implementation.

## Overview

The ImmoLux authentication system uses:

- **JWT tokens** (JSON Web Tokens) with HS256 signing
- **HTTP-only cookies** for secure token storage
- **Session management** in PostgreSQL database for token revocation
- **Remember Me** functionality for extended sessions
- **IP-based rate limiting** with automatic blocking after failed attempts
- **Authentication audit logging** for security tracking

## Architecture

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐      ┌──────────────┐
│   Client    │─────▶│  HTTP Route  │─────▶│  Handler    │─────▶│  Repository  │
│  (Browser)  │◀─────│  (Fiber v3)  │◀─────│  (Auth.go)  │◀─────│  (Database)  │
└─────────────┘      └──────────────┘      └─────────────┘      └──────────────┘
      │                     │
      │                     ▼
      │              ┌──────────────┐
      │              │  RouteContext│
      │              │  (context.go)│
      │              └──────────────┘
      │                     │
      │                     ▼
      │              ┌──────────────┐
      │              │ JWT Utils    │
      │              │ (jwt.go)     │
      │              └──────────────┘
      │
      ▼
  HTTP-only Cookie
  (session_token)
```

## API Endpoints

### Base URL

All endpoints are prefixed with `/api` (configured in `internal/server/server.go:21`)

### 1. Login (Public)

**Endpoint**: `POST /api/login`
**File**: `internal/server/routes/auth.go:15-97`

**Request Body**:

```json
{
  "email": "user@example.com",
  "password": "userpassword",
  "rememberMe": false
}
```

**Success Response** (200 OK):

```json
{
  "success": true,
  "data": {
    "user": {
      "id": 1,
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "phone": "+1234567890",
      "role": "user"
    }
  },
  "error": null
}
```

**Response Headers**:

- `Set-Cookie: session_token=<JWT>; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=<expiration>`

**Error Responses**:

| Status | Error Code            | Message                                                        | Reason                                         |
|--------|-----------------------|----------------------------------------------------------------|------------------------------------------------|
| 400    | BAD_REQUEST           | Invalid request body                                           | Malformed JSON or missing required fields      |
| 401    | INVALID_CREDENTIALS   | Invalid email or password                                      | Wrong email or password                        |
| 429    | RATE_LIMIT_EXCEEDED   | Too many login attempts. Try again after [timestamp]           | 5+ failed attempts for email+IP pair           |
| 500    | INTERNAL_ERROR        | Failed to authenticate user                                    | Database or server error                       |

**Cookie Details**:

- **Name**: `session_token` (constant in `internal/config/constants.go:11`)
- **HttpOnly**: `true` (prevents JavaScript access)
- **Secure**: Configured in `configs/config.toml` (`cookie_secure`)
- **SameSite**: `Strict` (CSRF protection)
- **Max-Age**:
    - `3600` seconds (1 hour) if `rememberMe = false`
    - `2592000` seconds (30 days) if `rememberMe = true`

**Implementation Notes**:

- Password is verified using bcrypt (`internal/database/user_repository.go`)
- Rate limiting tracks failed login attempts by email+IP pair (`internal/database/rate_limit_repository.go`)
- Email+IP pair is blocked for 30 minutes after 5 failed attempts within 15-minute window
- Rate limit automatically expires and resets after block period
- Prevents both IP-based attacks and targeted account attacks
- Authentication events logged for audit trail (`internal/database/auth_log_repository.go`)
- Session is created in database with hashed token (`internal/database/session_repository.go`)
- JWT contains: `userId`, `sessionId`, `rememberMe` claims

### 2. Is Connected (Public)

**Endpoint**: `GET /api/isconnected`
**File**: `internal/server/routes/auth.go:99-115`

**Request**: No body required, authentication is checked via cookie

**Success Response - Authenticated** (200 OK):

```json
{
  "success": true,
  "data": {
    "isConnected": true,
    "user": {
      "id": 1,
      "email": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "phone": "+1234567890",
      "role": "user"
    }
  },
  "error": null
}
```

**Success Response - Not Authenticated** (200 OK):

```json
{
  "success": true,
  "data": {
    "isConnected": false
  },
  "error": null
}
```

**Implementation Notes**:

- This endpoint does NOT return 401 errors
- Always returns 200 OK with `isConnected` boolean
- If authenticated, includes full user data
- Used for checking authentication status on page load

### 3. Logout (Secured)

**Endpoint**: `POST /api/logout`
**File**: `internal/server/routes/auth.go:117-146`

**Request**: No body required, uses session cookie for authentication

**Success Response** (200 OK):

```json
{
  "success": true,
  "data": {
    "success": true,
    "message": "Successfully logged out"
  },
  "error": null
}
```

**Response Headers**:

- `Set-Cookie: session_token=; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=0` (clears cookie)

**Error Responses**:

| Status | Error Code        | Message                                    | Reason                              |
|--------|-------------------|--------------------------------------------|-------------------------------------|
| 401    | COOKIE_MISSING    | Session cookie not found                   | No session_token cookie             |
| 401    | JWT_EXPIRED       | Session token has expired                  | JWT token expired                   |
| 401    | JWT_INVALID       | Session token is invalid                   | Invalid JWT signature               |
| 401    | SESSION_NOT_FOUND | Session not found                          | Session ID from JWT not in database |
| 401    | SESSION_INVALID   | Session is expired or has been invalidated | Session was revoked or expired      |
| 500    | INTERNAL_ERROR    | Failed to logout                           | Database error during invalidation  |

**Implementation Notes**:

- This is a **secured route** (requires authentication)
- Session is invalidated in database with reason "USER_LOGOUT" (`internal/database/session_repository.go:77-92`)
- Cookie is cleared using `ClearCookie()` method
- Subsequent requests with same JWT will fail authentication

## Authentication Flow Details

### Login Flow

1. **Request Validation** (`auth.go:39-51`)
    - Parse JSON body into `LoginPayload` struct
    - Validate required fields: `email`, `password`
    - Validate email format
    - Validate password minimum length

2. **Rate Limit Check** (`auth.go:56-66`)
    - Extract email, IP address, and User-Agent from request
    - Check rate limit for email+IP pair (`rate_limit_repository.go:26-114`)
    - If email+IP pair is blocked: return `RATE_LIMIT_EXCEEDED` with `blockedUntil` timestamp
    - Rate limiting rules:
        - Tracks by email + IP address combination
        - 15-minute sliding window
        - Maximum 5 attempts per window
        - 30-minute block after exceeding limit
        - Block automatically expires after 30 minutes
        - Prevents both brute force on single account and attacks trying multiple accounts from same IP

3. **User Authentication** (`auth.go:68-93`)
    - Query user by email (`user_repository.go:115-159`)
    - If user not found: return "Invalid email or password"
    - Verify password using bcrypt
    - If password invalid:
        - Log `LOGIN_FAILED` event to audit log
        - Return "Invalid email or password" error
        - Rate limiter tracks the failed attempt

4. **Session Creation** (`auth.go:99-159`)
    - Begin database transaction
    - Calculate expiration time based on `rememberMe`
    - Generate token hash from user ID, timestamp, and IP
    - Create session in database with:
        - User ID
        - Token hash (not plain JWT)
        - Remember me flag
        - Expiration timestamp
        - IP address (from request)
        - User agent (from request)
    - Generate JWT token with claims:
        - `userId`: User's database ID
        - `sessionId`: New session ID
        - `rememberMe`: Boolean from request
        - `iat`: Issued at timestamp
        - `exp`: Expiration timestamp (1 hour or 30 days)
        - `iss`: Issuer from config

5. **Response** (`auth.go:131-154`)
    - Reset rate limit for email+IP pair (successful login)
    - Log `LOGIN_SUCCESS` event to audit log
    - Create HTTP-only cookie with JWT
    - Return user data (without password hash)
    - Set cookie in response headers

### Authentication Check Flow

This flow happens automatically for secured routes and in `IsConnected` endpoint.

1. **Cookie Extraction** (`context.go:116-122`)
    - Extract `session_token` cookie from request
    - If missing: return `COOKIE_MISSING` error

2. **JWT Validation** (`context.go:124-145`)
    - Parse JWT token (`jwt.go:26-44`)
    - Verify signature using secret from config
    - Check expiration timestamp
    - Extract claims: `userId`, `sessionId`, `rememberMe`
    - If expired: return `JWT_EXPIRED` error
    - If invalid signature: return `JWT_INVALID` error

3. **Session Validation** (`context.go:147-171`)
    - Query session from database by `sessionId` (`session_repository.go:52-75`)
    - Check if session exists
    - If not found: return `SESSION_NOT_FOUND` error
    - Check if session is active (`is_active = true`)
    - Check if session is not expired (`expires_at > NOW()`)
    - If invalid: return `SESSION_INVALID` error

4. **Context Creation** (`context.go:173-178`)
    - Create `UserContext` with `userId` and `sessionId`
    - Store in `RouteContext` for handler access
    - No `authError` if successful

5. **Route Authorization** (`server.go:125-131`)
    - For secured routes, check `IsAuthenticated()`
    - If not authenticated, return specific error from `GetAuthError()`
    - If authenticated, proceed to handler

### Logout Flow

1. **Authentication Check** (`server.go:125-131`)
    - Secured route automatically validates session
    - If invalid, returns appropriate 401 error

2. **Session Invalidation** (`auth.go:125-133`)
    - Get session ID from context
    - Begin database transaction
    - Update session in database:
        - Set `is_active = false`
        - Set `invalidated_at = NOW()`
        - Set `invalidation_reason = "USER_LOGOUT"`

3. **Cookie Clearing** (`auth.go:135-136`)
    - Call `ClearCookie()` with cookie name
    - Sets `Max-Age=0` to delete cookie

4. **Response** (`auth.go:138-142`)
    - Return success message
    - Cookie is removed from browser

## Security Features

### 1. HTTP-Only Cookies

**File**: `internal/server/routes/auth.go:148-159`

- Cookies are marked `HttpOnly`, preventing JavaScript access
- Protects against XSS (Cross-Site Scripting) attacks
- Token cannot be stolen via malicious scripts

### 2. CSRF Protection

**File**: `internal/server/routes/auth.go:156`

- Cookies use `SameSite=Strict`
- Browser only sends cookie for same-site requests
- Prevents Cross-Site Request Forgery attacks

### 3. Secure Flag

**File**: `configs/config.toml.template:23`

- Configurable via `cookie_secure` in config
- Should be `true` in production with HTTPS
- Ensures cookie only sent over encrypted connections

### 4. Password Security

**File**: `internal/database/user_repository.go:66`

- Passwords stored as bcrypt hashes
- Uses `golang.org/x/crypto/bcrypt`
- Hashes are never sent to client

### 5. Email+IP Rate Limiting

**Files**:
- `internal/database/rate_limit_repository.go:26-149`
- `internal/database/ent/schema/rate_limit.go`

- Tracks failed login attempts by email + IP address combination
- 15-minute sliding window for attempt tracking
- Automatically blocks email+IP pair after 5 failures within window
- 30-minute block duration
- Block automatically expires and resets after block period
- Prevents brute force attacks on specific accounts from specific IPs
- Also prevents attackers from trying multiple accounts from same IP
- Reset on successful login
- More granular security than IP-only or account-only rate limiting

### 6. Authentication Audit Logging

**Files**:
- `internal/database/auth_log_repository.go`
- `internal/database/ent/schema/auth_log.go`

- All authentication events are logged for security auditing
- Tracked events:
    - `LOGIN_SUCCESS`: Successful login
    - `LOGIN_FAILED`: Failed password attempt
    - `LOGIN_BLOCKED`: IP blocked due to rate limit
    - `LOGOUT`: User logout
- Logs include:
    - User ID and email
    - IP address and User-Agent
    - Timestamp
    - Failure reason (for failed attempts)
- Helps detect security threats and suspicious activity

### 7. Session Revocation

**Files**:

- `internal/database/session_repository.go`
- `internal/database/ent/schema/session.go`

- Sessions stored in database, not just JWT
- Can be invalidated server-side
- Logout immediately invalidates session
- Expired sessions automatically fail validation
- Multiple active sessions supported per user
- Users can view and revoke individual sessions

### 8. Token Hashing

**File**: `internal/utils/hash.go`

- JWT tokens are hashed (SHA256) before database storage
- Plain JWT never stored in database
- If database compromised, tokens cannot be used

### 9. Specific Error Messages

**File**: `internal/server/routes/context.go`

- Different error codes for different failure reasons
- Helps frontend provide better UX
- Error codes:
    - `COOKIE_MISSING`: No session cookie
    - `JWT_EXPIRED`: Token expired
    - `JWT_INVALID`: Invalid signature
    - `SESSION_NOT_FOUND`: Session deleted
    - `SESSION_INVALID`: Session revoked/expired
    - `RATE_LIMIT_EXCEEDED`: Too many failed login attempts

## Database Schema

### Users Table

**File**: `internal/database/ent/schema/user.go`

Relevant fields for authentication:

```go
field.String("email").Unique()
field.String("first_name")
field.String("last_name")
field.Bool("is_active").Default(true)
```

### UserAuth Table

**File**: `internal/database/ent/schema/userauth.go`

```go
field.Int("user_id").Unique().Positive()
field.String("hash").NotEmpty().Sensitive() // bcrypt hash
field.Time("password_changed_at").Optional().Nillable()

// Foreign key
edge.From("user", User.Type).Ref("auth").Unique().Required()
```

### Sessions Table

**File**: `internal/database/ent/schema/session.go`

```go
field.String("session_token") // SHA256 hash of JWT
field.Bool("is_active").Default(true)
field.Bool("remember_me").Default(false)
field.Time("expires_at")
field.String("ip_address").Optional().Nillable()
field.String("user_agent").Optional().Nillable()
field.Time("invalidated_at").Optional().Nillable()
field.String("invalidation_reason").Optional().Nillable()

// Foreign key
edge.From("user", User.Type).Ref("sessions").Unique().Required()
```

### RateLimit Table

**File**: `internal/database/ent/schema/rate_limit.go`

```go
field.String("email").MaxLen(255)
field.String("ip_address").MaxLen(45)
field.String("action").MaxLen(50) // LOGIN, REGISTER, etc.
field.Int("attempt_count").Default(1)
field.Time("window_start").Default(time.Now)
field.Time("blocked_until").Optional().Nillable()

// Unique index on email + ip_address + action
index.Fields("email", "ip_address", "action").Unique()
```

### AuthLog Table

**File**: `internal/database/ent/schema/auth_log.go`

```go
field.Int("user_id").Optional().Nillable()
field.String("email")
field.String("event_type") // LOGIN_SUCCESS, LOGIN_FAILED, LOGIN_BLOCKED, LOGOUT
field.String("ip_address").Optional().Nillable()
field.String("user_agent").Optional().Nillable()
field.String("failure_reason").Optional().Nillable()

// Foreign key (optional, for failed attempts before user lookup)
edge.From("user", User.Type).Ref("auth_logs").Unique().Field("user_id")
```

## Configuration

### Config File

**File**: `configs/config.toml.template:20-23`

```toml
[security]
jwt_secret = "your-super-secret-jwt-key-change-in-production-min-32-chars"
jwt_issuer = "immolux"
cookie_secure = false  # Set to true in production with HTTPS
```

### Constants

**File**: `internal/config/constants.go:11`

```go
const SessionCookieName = "session_token"
```

## Key Files Reference

### Configuration

- `configs/config.toml.template` - Configuration template with security settings
- `internal/config/config.go:22-37` - SecurityConfig struct and methods
- `internal/config/constants.go:11` - SessionCookieName constant

### Database

- `internal/database/ent/schema/user.go` - User entity schema
- `internal/database/ent/schema/userauth.go` - User authentication schema
- `internal/database/ent/schema/session.go` - Session entity schema
- `internal/database/ent/schema/rate_limit.go` - Rate limiting schema
- `internal/database/ent/schema/auth_log.go` - Authentication audit log schema
- `internal/database/user_repository.go` - User authentication operations
- `internal/database/session_repository.go` - Session CRUD operations
- `internal/database/rate_limit_repository.go` - Rate limiting operations
- `internal/database/auth_log_repository.go` - Audit logging operations

### Authentication Logic

- `internal/server/routes/auth.go` - Login, IsConnected, Logout handlers
- `internal/server/routes/session.go` - Session management (view/revoke sessions)
- `internal/server/routes/context.go` - RouteContext and authentication extraction
- `internal/utils/jwt.go` - JWT generation and validation
- `internal/utils/hash.go` - Token hashing utilities
- `internal/utils/session.go` - Session expiration calculation

### Server

- `internal/server/server.go:75-139` - Route handlers with public/secured distinction
- `internal/server/routing.go:9-18` - Route registration

### Models

- `internal/models/auth.go` - DTOs for authentication (LoginRequest, UserDTO, etc.)
- `internal/models/server_api.go` - API response structure

### Error Handling

- `internal/server_error/server_error.go:54-57` - ServerError with Contains method

## Frontend Implementation Guide

### 1. Login Implementation

```typescript
// Login function
async function login(email: string, password: string, rememberMe: boolean) {
  const response = await fetch('/api/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include', // Important: allows cookies
    body: JSON.stringify({ email, password, rememberMe }),
  });

  const data = await response.json();

  if (!data.success) {
    // Handle errors
    switch (data.error.code) {
      case 'INVALID_CREDENTIALS':
        throw new Error('Invalid email or password');
      case 'RATE_LIMIT_EXCEEDED':
        // Error message contains the blockedUntil timestamp
        throw new Error(data.error.message);
      default:
        throw new Error(data.error.message);
    }
  }

  // Success - cookie is automatically stored by browser
  return data.data.user;
}
```

### 2. Check Authentication Status

```typescript
// Check if user is authenticated (on app load)
async function checkAuth() {
  const response = await fetch('/api/isconnected', {
    method: 'GET',
    credentials: 'include', // Important: sends cookie
  });

  const data = await response.json();

  if (data.success && data.data.isConnected) {
    return data.data.user; // User is authenticated
  }

  return null; // User is not authenticated
}
```

### 3. Logout Implementation

```typescript
// Logout function
async function logout() {
  const response = await fetch('/api/logout', {
    method: 'POST',
    credentials: 'include', // Important: sends cookie
  });

  const data = await response.json();

  if (!data.success) {
    // Handle errors
    switch (data.error.code) {
      case 'COOKIE_MISSING':
      case 'JWT_EXPIRED':
      case 'SESSION_INVALID':
        // User already logged out, just clear local state
        break;
      default:
        throw new Error(data.error.message);
    }
  }

  // Cookie is automatically cleared by browser
  return data.data;
}
```

### 4. Handle 401 Errors

```typescript
// API request wrapper with auth handling
async function apiRequest(url: string, options: RequestInit = {}) {
  const response = await fetch(url, {
    ...options,
    credentials: 'include', // Always include cookies
  });

  if (response.status === 401) {
    const data = await response.json();

    // Handle different authentication errors
    switch (data.error.code) {
      case 'COOKIE_MISSING':
        // User never logged in
        redirectToLogin('Please log in to continue');
        break;
      case 'JWT_EXPIRED':
        // Session expired
        redirectToLogin('Your session has expired. Please log in again.');
        break;
      case 'JWT_INVALID':
        // Invalid token
        redirectToLogin('Invalid session. Please log in again.');
        break;
      case 'SESSION_INVALID':
        // Session was revoked
        redirectToLogin('Your session is no longer valid. Please log in again.');
        break;
      case 'SESSION_NOT_FOUND':
        // Session deleted from database
        redirectToLogin('Session not found. Please log in again.');
        break;
    }

    throw new Error(data.error.message);
  }

  return response;
}
```

### 5. Protected Routes

```typescript
// React example with React Router
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean | null>(null);
  const [user, setUser] = useState(null);

  useEffect(() => {
    checkAuth()
      .then((user) => {
        if (user) {
          setUser(user);
          setIsAuthenticated(true);
        } else {
          setIsAuthenticated(false);
        }
      })
      .catch(() => {
        setIsAuthenticated(false);
      });
  }, []);

  if (isAuthenticated === null) {
    return <div>Loading...</div>;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return <>{children}</>;
}
```

### Important Frontend Notes

1. **Always use `credentials: 'include'`** in fetch requests to send/receive cookies
2. **Never try to access the JWT token** - it's HTTP-only and inaccessible to JavaScript
3. **Handle 401 errors globally** - redirect to login page
4. **Check authentication on app load** - use `/api/isconnected` endpoint
5. **Remember Me checkbox** - affects session duration (1 hour vs 30 days)
6. **Error codes are specific** - provide user-friendly messages based on error code
7. **Logout is instant** - server invalidates session immediately

## Testing Authentication

### Manual Testing with cURL

```bash
# Login
curl -X POST http://localhost:3000/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password","rememberMe":false}' \
  -c cookies.txt

# Check authentication
curl -X GET http://localhost:3000/api/isconnected \
  -b cookies.txt

# Logout
curl -X POST http://localhost:3000/api/logout \
  -b cookies.txt \
  -c cookies.txt
```

### Testing Remember Me

```bash
# Short session (1 hour)
curl -X POST http://localhost:3000/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password","rememberMe":false}' \
  -v

# Long session (30 days)
curl -X POST http://localhost:3000/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password","rememberMe":true}' \
  -v
```

Check the `Set-Cookie` header for `Max-Age` value.

## Troubleshooting

### Cookie Not Being Set

1. Check `cookie_secure` setting in `config.toml`
2. If `cookie_secure = true`, must use HTTPS
3. Ensure `credentials: 'include'` in frontend fetch

### 401 on Every Request

1. Check browser console for cookie
2. Verify `credentials: 'include'` in fetch
3. Check CORS settings if frontend on different domain
4. Verify JWT secret matches in config

### Session Expired Too Quickly

1. Check `rememberMe` value in login request
2. Verify expiration calculation in `internal/utils/session.go:14-21`
3. Check database `expires_at` field

### Email+IP Blocked (Rate Limited)

1. Check `rate_limit` table for email + IP address combination
2. Verify `blocked_until` timestamp
3. Block automatically expires after 30 minutes
4. Reset manually: `DELETE FROM rate_limit WHERE email = 'user@example.com' AND ip_address = '192.168.1.1' AND action = 'LOGIN'`
5. Check authentication audit logs for suspicious activity
6. Note: Same email from different IP can still attempt login
7. Note: Same IP with different email can still attempt login

## Implemented Features

Current authentication system includes:

1. ✅ **JWT Authentication**: Secure token-based authentication
2. ✅ **HTTP-Only Cookies**: XSS protection
3. ✅ **Session Management**: Database-backed sessions with revocation
4. ✅ **Multiple Sessions**: Users can have multiple active sessions
5. ✅ **Session UI**: View and revoke individual sessions
6. ✅ **Email+IP Rate Limiting**: Automatic blocking of email+IP pairs after 5 failed attempts
7. ✅ **Auto-Unlock**: Blocks automatically expire after 30 minutes
8. ✅ **Audit Logging**: Comprehensive authentication event tracking
9. ✅ **Remember Me**: Extended sessions (30 days vs 1 hour)
10. ✅ **Bcrypt Hashing**: Secure password storage

## Future Enhancements

Possible improvements to consider:

1. **Refresh Tokens**: Implement refresh token mechanism for seamless re-authentication
2. **Password Reset**: Email-based password reset flow
3. **Two-Factor Authentication**: TOTP or SMS-based 2FA
4. **Email Verification**: Require email verification before login
5. **OAuth Integration**: Google, Facebook, etc. login
6. **Geolocation Tracking**: Track login locations for security
7. **Device Fingerprinting**: Identify trusted devices
8. **Anomaly Detection**: Machine learning for suspicious login patterns
