package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"silo/pkg/gateway/models"
)

func newTestServer(token string) *server {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	s := &server{
		cfg: models.ServerConfig{
			Host:  "127.0.0.1",
			Port:  0,
			Token: token,
		},
		engine:    engine,
		startTime: time.Now(),
	}
	s.registerRoutes()
	return s
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
