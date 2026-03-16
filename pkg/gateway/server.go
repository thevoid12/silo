package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"silo/pkg/gateway/models"
	"silo/version"
)

type server struct {
	cfg       models.ServerConfig
	deps      models.ServerDeps
	engine    *gin.Engine
	httpSrv   *http.Server
	startTime time.Time
}

// New creates a new GatewayServer with configuration, runtime dependencies, and a logger
func New(cfg models.ServerConfig, deps models.ServerDeps, log *zap.Logger) models.GatewayServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	if log == nil {
		log = zap.NewNop()
	}
	engine.Use(InjectLogger(log))

	s := &server{cfg: cfg, deps: deps, engine: engine}
	s.registerRoutes()
	return s
}

func (s *server) registerRoutes() {
	s.engine.GET("/health", s.handleHealth)

	auth := s.engine.Group("/silo")
	auth.Use(BearerAuth(s.cfg.Token))
	auth.GET("/status", s.handleStatus)
	auth.POST("/brain/chat", s.handleChat)
	auth.POST("/brain/tool-approval", s.handleToolApproval)
}

func (s *server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{Status: "ok"})
}

func (s *server) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, models.StatusResponse{
		Status:  "running",
		Uptime:  time.Since(s.startTime).Round(time.Second).String(),
		Version: version.VERSION,
		Host:    s.cfg.Host,
		Port:    s.cfg.Port,
	})
}

// Run starts the HTTP server (blocking until shutdown or error).
// WriteTimeout is 0 to support long-lived SSE streams on /silo/brain/chat.
func (s *server) Run() error {
	s.startTime = time.Now()
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpSrv = &http.Server{
		Addr:        addr,
		Handler:     s.engine,
		ReadTimeout: s.cfg.ReadTimeout,
		IdleTimeout: s.cfg.IdleTimeout,
		// WriteTimeout intentionally 0: SSE streams require no write deadline
	}
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server
func (s *server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}
