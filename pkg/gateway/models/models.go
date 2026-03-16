package models

import (
	"context"
	"time"
)

// GatewayServer is the interface for the HTTP gateway server
type GatewayServer interface {
	Run() error
	Shutdown(ctx context.Context) error
}

// ServerConfig holds the gateway server configuration
type ServerConfig struct {
	Host         string
	Port         int
	Token        string
	PIDFile      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// StatusResponse is the response body for GET /silo/status
type StatusResponse struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

// HealthResponse is the response body for GET /health
type HealthResponse struct {
	Status string `json:"status"`
}
