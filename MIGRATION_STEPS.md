# PostgreSQL Migration - Complete Guide

## Quick Start

### 1. Install Dependencies
```bash
go mod tidy
```

### 2. Generate Ent Client Code
```bash
go generate ./internal/database/ent
```

### 3. Configure Database Connection

Edit `configs/config.toml` and update the database section:

```toml
[database]
max_idle_connections = 50
max_open_connections = 50
user_file_path = "configs/standardUserList.json"
host = "localhost"
port = 5432
username = "postgres"
password = "your_password_here"
db_name = "immolux"
ssl_mode = "disable"  # Use "require" for production
```

### 4. Create PostgreSQL Database

```sql
CREATE DATABASE immolux;
```

### 5. Build and Run

```bash
go build -o immo-lux-server ./internal
./immo-lux-server
```

Or use the "Server" run configuration in your IDE.

---

## Migration Summary

### ✅ Database Layer
- **Migrated from**: dbmate with SQLite
- **Migrated to**: Ent ORM with PostgreSQL
- Added connection pooling configuration
- Implemented automatic schema migration
- Added proper resource cleanup and error handling

### ✅ Ent Schemas
- **User**: Core user entity with email validation and constraints
  - Fields: first_name, last_name, email (unique, indexed), is_active
  - Max lengths: names (100), email (255)

- **UserAuth**: Authentication and security entity
  - Fields: user_id (unique), hash (sensitive), is_locked, failed_login_attempts
  - Password hashes marked as sensitive (not logged)
  - Failed attempts tracked with timestamps

- **Session**: User session management
  - Fields: session_token (unique, sensitive), expires_at, is_active
  - Tracks IP address, user agent, and invalidation reason
  - Composite index on (user_id, is_active) for fast queries

### ✅ Configuration
- **No environment variables needed** - All settings in `config.toml`
- Connection string automatically built from config parameters
- Configurable SSL mode (disable/require/verify-ca/verify-full)
- Connection pool settings: max_idle_connections, max_open_connections

### ✅ Code Quality Improvements
- Fixed resource leaks (proper logger cleanup on errors)
- Removed redundant else statements
- Improved error messages with context
- Added constraint validation (email uniqueness, positive IDs, etc.)
- Better logging with debug/error levels
- Sensitive data (passwords, tokens) marked and protected

### ✅ Removed
- `internal/database/modern_sqlite_driver.go`
- `internal/database/migrations/` directory
- `db/immolux.db` SQLite database file
- dbmate dependency
- SQLite driver dependency

---

## Schema Features

### Foreign Keys
- UserAuth.user_id → User.id (CASCADE on delete)
- Session.user_id → User.id (CASCADE on delete)

### Indexes
- User: email (unique)
- UserAuth: user_id (unique)
- Session: (user_id, is_active), session_token (unique)

### Validation
- Email uniqueness enforced
- Non-negative failed login attempts
- Positive user IDs
- Max field lengths for all strings
- Required fields validated

### Security
- Password hashes stored with bcrypt
- Sensitive fields (hash, session_token) marked appropriately
- Failed login attempts tracked with timestamps
- Session expiration and invalidation supported

---

## Next Steps

1. **Generate Ent code**: `go generate ./internal/database/ent`
2. **Configure database**: Edit `configs/config.toml`
3. **Create database**: `CREATE DATABASE immolux;`
4. **Run application**: Schema auto-migrates on startup
5. **Monitor logs**: Check `logs/DATABASE.log` for details

## Production Recommendations

- Change `ssl_mode` to `require` or `verify-full`
- Use strong passwords in `config.toml`
- Consider using environment variables or secrets management for passwords
- Adjust connection pool settings based on load
- Enable PostgreSQL connection logging for monitoring
- Set up database backups
- Use a dedicated database user with limited permissions
