package core

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/adk/session"

	siloDb "silo/pkg/db"
)

func openTestDB(t *testing.T) session.Service {
	t.Helper()
	database, err := siloDb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	return NewSQLiteSessionService(database)
}

func TestSQLiteSessionService_CreateGet(t *testing.T) {
	ctx := context.Background()
	svc := openTestDB(t)

	resp, err := svc.Create(ctx, &session.CreateRequest{AppName: "silo", UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resp.Session == nil {
		t.Fatal("expected non-nil session")
	}

	got, err := svc.Get(ctx, &session.GetRequest{
		AppName: "silo", UserID: "u1", SessionID: resp.Session.ID(),
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Session.ID() != resp.Session.ID() {
		t.Fatalf("session ID mismatch: want %s got %s", resp.Session.ID(), got.Session.ID())
	}
}

func TestSQLiteSessionService_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")
	ctx := context.Background()

	db1, err := siloDb.Open(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	svc1 := NewSQLiteSessionService(db1)
	resp, err := svc1.Create(ctx, &session.CreateRequest{AppName: "silo", UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sid := resp.Session.ID()

	db2, err := siloDb.Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	svc2 := NewSQLiteSessionService(db2)
	got, err := svc2.Get(ctx, &session.GetRequest{AppName: "silo", UserID: "u1", SessionID: sid})
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Session.ID() != sid {
		t.Fatalf("session not persisted: want %s got %s", sid, got.Session.ID())
	}
}

func TestSQLiteSessionService_AppendEvent(t *testing.T) {
	ctx := context.Background()
	svc := openTestDB(t)

	resp, err := svc.Create(ctx, &session.CreateRequest{AppName: "silo", UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sess := resp.Session

	evt := session.NewEvent("inv-1")
	evt.Author = "user"
	if err := svc.AppendEvent(ctx, sess, evt); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	got, err := svc.Get(ctx, &session.GetRequest{
		AppName: "silo", UserID: "u1", SessionID: sess.ID(),
	})
	if err != nil {
		t.Fatalf("Get after AppendEvent: %v", err)
	}
	if got.Session.Events().Len() != 1 {
		t.Fatalf("expected 1 event, got %d", got.Session.Events().Len())
	}
}

func TestSQLiteSessionService_Delete(t *testing.T) {
	ctx := context.Background()
	svc := openTestDB(t)

	resp, _ := svc.Create(ctx, &session.CreateRequest{AppName: "silo", UserID: "u1"})
	sid := resp.Session.ID()

	if err := svc.Delete(ctx, &session.DeleteRequest{AppName: "silo", UserID: "u1", SessionID: sid}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Get(ctx, &session.GetRequest{AppName: "silo", UserID: "u1", SessionID: sid}); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestSQLiteSessionService_List(t *testing.T) {
	ctx := context.Background()
	svc := openTestDB(t)

	for i := range 3 {
		_, err := svc.Create(ctx, &session.CreateRequest{
			AppName: "silo", UserID: "u1", SessionID: string(rune('a' + i)),
		})
		if err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	listResp, err := svc.List(ctx, &session.ListRequest{AppName: "silo", UserID: "u1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listResp.Sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(listResp.Sessions))
	}
}

func TestSQLiteSessionService_BadPath(t *testing.T) {
	_, err := siloDb.Open("/nonexistent/dir/test.db")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}
