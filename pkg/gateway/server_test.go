package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/adk/session"

	"silo/pkg/approval"
	approvalmodels "silo/pkg/approval/models"
	gatewaymodels "silo/pkg/gateway/models"
)

func newTestServer(token string) *server {
	gin.SetMode(gin.TestMode)
	approvalSvc := approval.New(approvalmodels.ServiceConfig{Timeout: time.Second})
	cfg := gatewaymodels.ServerConfig{Host: "127.0.0.1", Port: 0, Token: token}
	deps := gatewaymodels.ServerDeps{
		Sessions:  session.InMemoryService(),
		Approval:  approvalSvc,
		NewRunner: nil, // chat handler tested separately
	}
	return New(cfg, deps, nil).(*server)
}

func TestHealth(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStatusUnauthorized(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/silo/status", nil)
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestStatusAuthorized(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/silo/status", nil)
	req.Header.Set("Authorization", "Bearer token")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStatusWrongToken(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/silo/status", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBearerAuthCaseInsensitive(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/silo/status", nil)
	req.Header.Set("Authorization", "bearer token")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; bearer scheme should be case-insensitive", w.Code)
	}
}

func TestChatMissingBody(t *testing.T) {
	s := newTestServer("token")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/chat", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatMissingMessage(t *testing.T) {
	s := newTestServer("token")
	body, _ := json.Marshal(map[string]string{"session_id": "abc"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChatUnauthorized(t *testing.T) {
	s := newTestServer("token")
	body, _ := json.Marshal(gatewaymodels.ChatRequest{Message: "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestToolApprovalUnauthorized(t *testing.T) {
	s := newTestServer("token")
	body, _ := json.Marshal(gatewaymodels.ToolApprovalRequest{RequestID: "x", Approved: true})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/tool-approval", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestToolApprovalNotFound(t *testing.T) {
	s := newTestServer("token")
	body, _ := json.Marshal(gatewaymodels.ToolApprovalRequest{RequestID: "no-such-id", Approved: true})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/tool-approval", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestToolApprovalSuccess(t *testing.T) {
	s := newTestServer("token")

	// Register a pending approval entry before responding
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		s.deps.Approval.Request(ctx, approvalmodels.ApprovalRequest{ //nolint:errcheck
			ID:      "req-1",
			Tool:    "shell",
			Command: "ls",
		})
	}()

	// Wait briefly for the request to register
	time.Sleep(50 * time.Millisecond)

	body, _ := json.Marshal(gatewaymodels.ToolApprovalRequest{RequestID: "req-1", Approved: true})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/tool-approval", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestToolApprovalMissingRequestID(t *testing.T) {
	s := newTestServer("token")
	body, _ := json.Marshal(map[string]bool{"approved": true})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/silo/brain/tool-approval", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
