-- migrate:up
PRAGMA foreign_keys = ON;

CREATE TABLE "user"
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT                 NOT NULL,
    email      TEXT UNIQUE          NOT NULL,
    is_active  INTEGER  DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "user_auth"
(
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                 INTEGER     NOT NULL,
    hash                    TEXT        NOT NULL,
    is_locked               INTEGER     DEFAULT 0,
    failed_login_attempts   INTEGER     DEFAULT 0,
    last_failed_attempt     DATETIME,
    created_at              DATETIME    DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME    DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES "user" (id)
);

CREATE INDEX idx_user_auth_user_id ON "user_auth" (user_id);

CREATE TABLE "user_profile"
(
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER     NOT NULL,
    first_name  TEXT,
    last_name   TEXT,
    height      REAL,
    weight      REAL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES "user" (id)
);

CREATE INDEX idx_user_profile_user_id ON "user_profile" (user_id);

CREATE TABLE "session"
(
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER     NOT NULL,
    is_active   INTEGER     DEFAULT 1,
    created_at  DATETIME    DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME    DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES "user" (id)
);

CREATE INDEX idx_session_user_id ON "session" (user_id);

-- migrate:down
PRAGMA foreign_keys = OFF;
DROP TABLE IF EXISTS "user_profile";
DROP TABLE IF EXISTS "user_auth";
DROP TABLE IF EXISTS "session";
DROP TABLE IF EXISTS "user";
PRAGMA foreign_keys = ON;

