package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"silo/pkg/gateway/models"
	"silo/version"
)

type server struct {
	cfg       models.ServerConfig
	engine    *gin.Engine
	httpSrv   *http.Server
	startTime time.Time
}

// New creates a new GatewayServer with the given configuration
func New(cfg models.ServerConfig) models.GatewayServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &server{cfg: cfg, engine: engine}
	s.registerRoutes()
	return s
}

func (s *server) registerRoutes() {
	s.engine.GET("/health", s.handleHealth)

	auth := s.engine.Group("/silo")
	auth.Use(BearerAuth(s.cfg.Token))
	auth.GET("/status", s.handleStatus)
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

// Run starts the HTTP server (blocking until shutdown or error)
func (s *server) Run() error {
	s.startTime = time.Now()
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.engine,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  s.cfg.IdleTimeout,
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
