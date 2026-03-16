-- name: CreateSession :exec
INSERT INTO sessions (session_id, app_name, user_id, state, created_at, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- name: GetSession :one
SELECT session_id, app_name, user_id, state, created_at, updated_at
FROM sessions
WHERE app_name = ? AND user_id = ? AND session_id = ?;

-- name: ListSessionsByApp :many
SELECT session_id, app_name, user_id, state, created_at, updated_at
FROM sessions
WHERE app_name = ?
ORDER BY updated_at DESC;

-- name: ListSessionsByUser :many
SELECT session_id, app_name, user_id, state, created_at, updated_at
FROM sessions
WHERE app_name = ? AND user_id = ?
ORDER BY updated_at DESC;

-- name: UpdateSessionState :exec
UPDATE sessions
SET state = ?, updated_at = CURRENT_TIMESTAMP
WHERE app_name = ? AND user_id = ? AND session_id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE app_name = ? AND user_id = ? AND session_id = ?;

-- name: InsertEvent :exec
INSERT INTO session_events (id, session_id, app_name, user_id, event_json, timestamp)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetEventsBySession :many
SELECT id, session_id, app_name, user_id, event_json, timestamp
FROM session_events
WHERE app_name = ? AND user_id = ? AND session_id = ?
ORDER BY timestamp ASC;

-- name: GetRecentEventsBySession :many
SELECT id, session_id, app_name, user_id, event_json, timestamp
FROM session_events
WHERE app_name = ? AND user_id = ? AND session_id = ?
ORDER BY timestamp DESC
LIMIT ?;

-- name: GetEventsAfterTimestamp :many
SELECT id, session_id, app_name, user_id, event_json, timestamp
FROM session_events
WHERE app_name = ? AND user_id = ? AND session_id = ? AND timestamp >= ?
ORDER BY timestamp ASC;

-- name: UpsertAppState :exec
INSERT INTO app_states (app_name, state, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(app_name) DO UPDATE SET
    state      = excluded.state,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetAppState :one
SELECT state FROM app_states WHERE app_name = ?;

-- name: UpsertUserState :exec
INSERT INTO user_states (app_name, user_id, state, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(app_name, user_id) DO UPDATE SET
    state      = excluded.state,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetUserState :one
SELECT state FROM user_states WHERE app_name = ? AND user_id = ?;
