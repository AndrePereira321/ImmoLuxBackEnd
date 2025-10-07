-- migrate:up
PRAGMA
foreign_keys = ON;

CREATE TABLE "user"
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT        NOT NULL,
    last_name  TEXT        NOT NULL,
    email      TEXT UNIQUE NOT NULL,
    is_active  INTEGER  DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "user_auth"
(
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id               INTEGER NOT NULL UNIQUE, -- Added UNIQUE
    hash                  TEXT    NOT NULL,
    is_locked             INTEGER  DEFAULT 0,
    locked_reason         TEXT,
    failed_login_attempts INTEGER  DEFAULT 0,
    last_failed_attempt   DATETIME,
    password_changed_at   DATETIME,
    created_at            DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES "user" (id) ON DELETE CASCADE
);

CREATE INDEX idx_user_auth_user_id ON "user_auth" (user_id);

CREATE TABLE "session"
(
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id            INTEGER  NOT NULL,
    session_token      TEXT     NOT NULL UNIQUE,
    is_active          INTEGER  DEFAULT 1,
    expires_at         DATETIME NOT NULL,
    invalidated_at     DATETIME,
    invalidated_reason TEXT,
    ip_address         TEXT,
    user_agent         TEXT,
    created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES "user" (id) ON DELETE CASCADE
);

CREATE INDEX idx_session_user_id_active ON "session" (user_id, is_active);
CREATE INDEX idx_session_token ON "session" (session_token);

-- migrate:down
PRAGMA
foreign_keys = OFF;
DROP TABLE IF EXISTS "user_auth";
DROP TABLE IF EXISTS "session";
DROP TABLE IF EXISTS "user";
PRAGMA
foreign_keys = ON;

