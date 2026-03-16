-- Sessions: one row per active conversation thread.
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT     NOT NULL,
    app_name   TEXT     NOT NULL,
    user_id    TEXT     NOT NULL,
    state      TEXT     NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (app_name, user_id, session_id)
);

-- Session events: each turn stored as a JSON blob.
CREATE TABLE IF NOT EXISTS session_events (
    id         TEXT     NOT NULL,
    session_id TEXT     NOT NULL,
    app_name   TEXT     NOT NULL,
    user_id    TEXT     NOT NULL,
    event_json TEXT     NOT NULL,
    timestamp  DATETIME NOT NULL,
    PRIMARY KEY (id, app_name, user_id, session_id),
    FOREIGN KEY (app_name, user_id, session_id)
        REFERENCES sessions(app_name, user_id, session_id) ON DELETE CASCADE
);

-- App-level state: shared across all users for a given app.
CREATE TABLE IF NOT EXISTS app_states (
    app_name   TEXT     NOT NULL PRIMARY KEY,
    state      TEXT     NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User-level state: shared across all sessions for a given user+app.
CREATE TABLE IF NOT EXISTS user_states (
    app_name   TEXT     NOT NULL,
    user_id    TEXT     NOT NULL,
    state      TEXT     NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (app_name, user_id)
);
