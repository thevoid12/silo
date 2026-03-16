package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/adk/session"

	"silo/pkg/db/dbal"
	coremodels "silo/pkg/core/models"
	siloerrors "silo/pkg/errors"
)

// NewSQLiteSessionService opens a SQLite-backed session service and runs schema migrations.
func NewSQLiteSessionService(cfg coremodels.SessionConfig) (session.Service, error) {
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", siloerrors.ErrDBOpen, err)
	}
	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: %s", siloerrors.ErrDBMigrate, err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer; serialise through one connection
	return &sqliteService{db: db, q: dbal.New(db)}, nil
}

// migrateSchema creates all required tables if they do not exist.
func migrateSchema(db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS sessions (
    session_id TEXT     NOT NULL,
    app_name   TEXT     NOT NULL,
    user_id    TEXT     NOT NULL,
    state      TEXT     NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (app_name, user_id, session_id)
);
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
CREATE TABLE IF NOT EXISTS app_states (
    app_name   TEXT     NOT NULL PRIMARY KEY,
    state      TEXT     NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS user_states (
    app_name   TEXT     NOT NULL,
    user_id    TEXT     NOT NULL,
    state      TEXT     NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (app_name, user_id)
);`
	_, err := db.Exec(ddl)
	return err
}

// sqliteService implements session.Service backed by SQLite.
type sqliteService struct {
	db *sql.DB
	q  *dbal.Queries
}

func (s *sqliteService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("app_name and user_id are required")
	}
	sid := req.SessionID
	if sid == "" {
		sid = uuid.NewString()
	}

	appDelta, userDelta, sessionState := extractStateDeltas(req.State)

	appState, err := s.loadAppState(ctx, req.AppName)
	if err != nil {
		return nil, err
	}
	userState, err := s.loadUserState(ctx, req.AppName, req.UserID)
	if err != nil {
		return nil, err
	}
	maps.Copy(appState, appDelta)
	maps.Copy(userState, userDelta)

	if err := s.persistAppState(ctx, req.AppName, appState); err != nil {
		return nil, err
	}
	if err := s.persistUserState(ctx, req.AppName, req.UserID, userState); err != nil {
		return nil, err
	}

	stateJSON, err := jsonMarshal(sessionState)
	if err != nil {
		return nil, err
	}
	if err := s.q.CreateSession(ctx, dbal.CreateSessionParams{
		SessionID: sid,
		AppName:   req.AppName,
		UserID:    req.UserID,
		State:     stateJSON,
	}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	merged := mergeStates(appState, userState, sessionState)
	return &session.CreateResponse{
		Session: &siloSession{
			sessionID: sid,
			appName:   req.AppName,
			userID:    req.UserID,
			state:     merged,
			updatedAt: time.Now(),
		},
	}, nil
}

func (s *sqliteService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return nil, fmt.Errorf("app_name, user_id, session_id are required")
	}

	row, err := s.q.GetSession(ctx, dbal.GetSessionParams{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	var sessionState map[string]any
	if err := json.Unmarshal([]byte(row.State), &sessionState); err != nil {
		sessionState = make(map[string]any)
	}

	appState, _ := s.loadAppState(ctx, req.AppName)
	userState, _ := s.loadUserState(ctx, req.AppName, req.UserID)

	evts, err := s.loadEvents(ctx, req)
	if err != nil {
		return nil, err
	}

	return &session.GetResponse{
		Session: &siloSession{
			sessionID: row.SessionID,
			appName:   row.AppName,
			userID:    row.UserID,
			state:     mergeStates(appState, userState, sessionState),
			events:    evts,
			updatedAt: row.UpdatedAt,
		},
	}, nil
}

func (s *sqliteService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("app_name is required")
	}

	var rows []dbal.Session
	var err error
	if req.UserID != "" {
		rows, err = s.q.ListSessionsByUser(ctx, dbal.ListSessionsByUserParams{
			AppName: req.AppName,
			UserID:  req.UserID,
		})
	} else {
		rows, err = s.q.ListSessionsByApp(ctx, req.AppName)
	}
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	appState, _ := s.loadAppState(ctx, req.AppName)

	sessions := make([]session.Session, 0, len(rows))
	for _, row := range rows {
		var sessionState map[string]any
		if err := json.Unmarshal([]byte(row.State), &sessionState); err != nil {
			sessionState = make(map[string]any)
		}
		userState, _ := s.loadUserState(ctx, req.AppName, row.UserID)
		sessions = append(sessions, &siloSession{
			sessionID: row.SessionID,
			appName:   row.AppName,
			userID:    row.UserID,
			state:     mergeStates(appState, userState, sessionState),
			updatedAt: row.UpdatedAt,
		})
	}
	return &session.ListResponse{Sessions: sessions}, nil
}

func (s *sqliteService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return fmt.Errorf("app_name, user_id, session_id are required")
	}
	return s.q.DeleteSession(ctx, dbal.DeleteSessionParams{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
}

func (s *sqliteService) AppendEvent(ctx context.Context, curSession session.Session, event *session.Event) error {
	if curSession == nil {
		return fmt.Errorf("session is nil")
	}
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	if event.Partial {
		return nil
	}

	sess, ok := curSession.(*siloSession)
	if !ok {
		return fmt.Errorf("unexpected session type %T", curSession)
	}

	event.Timestamp = event.Timestamp.Truncate(time.Microsecond)
	event = trimTempState(event)

	appDelta, userDelta, sessionDelta := extractStateDeltas(event.Actions.StateDelta)

	appState, err := s.loadAppState(ctx, sess.AppName())
	if err != nil {
		return err
	}
	userState, err := s.loadUserState(ctx, sess.AppName(), sess.UserID())
	if err != nil {
		return err
	}
	maps.Copy(appState, appDelta)
	maps.Copy(userState, userDelta)

	if err := s.persistAppState(ctx, sess.AppName(), appState); err != nil {
		return err
	}
	if err := s.persistUserState(ctx, sess.AppName(), sess.UserID(), userState); err != nil {
		return err
	}

	sess.mu.Lock()
	maps.Copy(sess.state, sessionDelta)
	sess.events = append(sess.events, event)
	sess.updatedAt = event.Timestamp
	sess.mu.Unlock()

	stateJSON, err := jsonMarshal(sess.state)
	if err != nil {
		return err
	}
	if err := s.q.UpdateSessionState(ctx, dbal.UpdateSessionStateParams{
		State:     stateJSON,
		AppName:   sess.AppName(),
		UserID:    sess.UserID(),
		SessionID: sess.ID(),
	}); err != nil {
		return fmt.Errorf("update session state: %w", err)
	}

	evtJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return s.q.InsertEvent(ctx, dbal.InsertEventParams{
		ID:        event.ID,
		SessionID: sess.ID(),
		AppName:   sess.AppName(),
		UserID:    sess.UserID(),
		EventJson: string(evtJSON),
		Timestamp: event.Timestamp,
	})
}

// loadEvents fetches events for a Get request applying NumRecentEvents / After filters.
func (s *sqliteService) loadEvents(ctx context.Context, req *session.GetRequest) ([]*session.Event, error) {
	var rows []dbal.SessionEvent
	var err error

	switch {
	case req.NumRecentEvents > 0:
		rows, err = s.q.GetRecentEventsBySession(ctx, dbal.GetRecentEventsBySessionParams{
			AppName:   req.AppName,
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Limit:     int64(req.NumRecentEvents),
		})
		if err == nil {
			// reverse: query returns DESC order
			for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	case !req.After.IsZero():
		rows, err = s.q.GetEventsAfterTimestamp(ctx, dbal.GetEventsAfterTimestampParams{
			AppName:   req.AppName,
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Timestamp: req.After,
		})
	default:
		rows, err = s.q.GetEventsBySession(ctx, dbal.GetEventsBySessionParams{
			AppName:   req.AppName,
			UserID:    req.UserID,
			SessionID: req.SessionID,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}

	evts := make([]*session.Event, 0, len(rows))
	for _, row := range rows {
		var evt session.Event
		if err := json.Unmarshal([]byte(row.EventJson), &evt); err != nil {
			return nil, fmt.Errorf("unmarshal event %s: %w", row.ID, err)
		}
		evts = append(evts, &evt)
	}
	return evts, nil
}

// loadAppState reads app state from DB; returns empty map if not found.
func (s *sqliteService) loadAppState(ctx context.Context, appName string) (map[string]any, error) {
	raw, err := s.q.GetAppState(ctx, appName)
	if err != nil {
		return make(map[string]any), nil // no row yet
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return make(map[string]any), nil
	}
	return m, nil
}

func (s *sqliteService) persistAppState(ctx context.Context, appName string, state map[string]any) error {
	raw, err := jsonMarshal(state)
	if err != nil {
		return err
	}
	return s.q.UpsertAppState(ctx, dbal.UpsertAppStateParams{AppName: appName, State: raw})
}

// loadUserState reads user state from DB; returns empty map if not found.
func (s *sqliteService) loadUserState(ctx context.Context, appName, userID string) (map[string]any, error) {
	raw, err := s.q.GetUserState(ctx, dbal.GetUserStateParams{AppName: appName, UserID: userID})
	if err != nil {
		return make(map[string]any), nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return make(map[string]any), nil
	}
	return m, nil
}

func (s *sqliteService) persistUserState(ctx context.Context, appName, userID string, state map[string]any) error {
	raw, err := jsonMarshal(state)
	if err != nil {
		return err
	}
	return s.q.UpsertUserState(ctx, dbal.UpsertUserStateParams{AppName: appName, UserID: userID, State: raw})
}

// ---- helpers ----------------------------------------------------------------

func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(b), nil
}

// extractStateDeltas splits a state map into app, user, and session portions by key prefix.
func extractStateDeltas(delta map[string]any) (appDelta, userDelta, sessionDelta map[string]any) {
	appDelta = make(map[string]any)
	userDelta = make(map[string]any)
	sessionDelta = make(map[string]any)
	for k, v := range delta {
		switch {
		case strings.HasPrefix(k, session.KeyPrefixApp):
			appDelta[strings.TrimPrefix(k, session.KeyPrefixApp)] = v
		case strings.HasPrefix(k, session.KeyPrefixUser):
			userDelta[strings.TrimPrefix(k, session.KeyPrefixUser)] = v
		case !strings.HasPrefix(k, session.KeyPrefixTemp):
			sessionDelta[k] = v
		}
	}
	return
}

// mergeStates recombines app, user, and session state into a single map with prefixes restored.
func mergeStates(appState, userState, sessionState map[string]any) map[string]any {
	merged := make(map[string]any, len(appState)+len(userState)+len(sessionState))
	maps.Copy(merged, sessionState)
	for k, v := range appState {
		merged[session.KeyPrefixApp+k] = v
	}
	for k, v := range userState {
		merged[session.KeyPrefixUser+k] = v
	}
	return merged
}

// trimTempState removes temp: keys from event state delta before persistence.
func trimTempState(event *session.Event) *session.Event {
	if len(event.Actions.StateDelta) == 0 {
		return event
	}
	filtered := make(map[string]any, len(event.Actions.StateDelta))
	for k, v := range event.Actions.StateDelta {
		if !strings.HasPrefix(k, session.KeyPrefixTemp) {
			filtered[k] = v
		}
	}
	event.Actions.StateDelta = filtered
	return event
}

// ---- siloSession: implements session.Session --------------------------------

type siloSession struct {
	mu        sync.RWMutex
	sessionID string
	appName   string
	userID    string
	state     map[string]any
	events    []*session.Event
	updatedAt time.Time
}

func (s *siloSession) ID() string      { return s.sessionID }
func (s *siloSession) AppName() string { return s.appName }
func (s *siloSession) UserID() string  { return s.userID }

func (s *siloSession) State() session.State {
	return &siloState{mu: &s.mu, state: s.state}
}

func (s *siloSession) Events() session.Events {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return siloEvents(s.events)
}

func (s *siloSession) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

// ---- siloState --------------------------------------------------------------

type siloState struct {
	mu    *sync.RWMutex
	state map[string]any
}

func (s *siloState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	if !ok {
		return nil, session.ErrStateKeyNotExist
	}
	return v, nil
}

func (s *siloState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
	return nil
}

func (s *siloState) All() iter.Seq2[string, any] {
	s.mu.RLock()
	cp := maps.Clone(s.state)
	s.mu.RUnlock()
	return func(yield func(string, any) bool) {
		for k, v := range cp {
			if !yield(k, v) {
				return
			}
		}
	}
}

// ---- siloEvents -------------------------------------------------------------

type siloEvents []*session.Event

func (e siloEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, ev := range e {
			if !yield(ev) {
				return
			}
		}
	}
}

func (e siloEvents) Len() int              { return len(e) }
func (e siloEvents) At(i int) *session.Event {
	if i >= 0 && i < len(e) {
		return e[i]
	}
	return nil
}

var (
	_ session.Service = (*sqliteService)(nil)
	_ session.Session = (*siloSession)(nil)
	_ session.State   = (*siloState)(nil)
	_ session.Events  = (siloEvents)(nil)
)
