package models

import (
	"context"
	"time"

	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	approvalmodels "silo/pkg/approval/models"
)

// GatewayServer is the interface for the HTTP gateway server
type GatewayServer interface {
	Run() error
	Shutdown(ctx context.Context) error
}

// RunnerFactory creates a new ADK runner per chat request, using the server's shared approval service
type RunnerFactory func(ctx context.Context) (*runner.Runner, error)

// ServerConfig holds the gateway HTTP server configuration
type ServerConfig struct {
	Host         string
	Port         int
	Token        string
	PIDFile      string
	ReadTimeout  time.Duration
	// WriteTimeout should be 0 for SSE streaming; set non-zero only if SSE is not used
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// ServerDeps holds runtime dependencies injected into the gateway server
type ServerDeps struct {
	Sessions  session.Service
	Approval  approvalmodels.ApprovalService
	NewRunner RunnerFactory
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

// ChatRequest is the request body for POST /silo/brain/chat
type ChatRequest struct {
	Message   string `json:"message" binding:"required"`
	SessionID string `json:"session_id"`
}

// ToolApprovalRequest is the request body for POST /silo/brain/tool-approval
type ToolApprovalRequest struct {
	RequestID string `json:"request_id" binding:"required"`
	Approved  bool   `json:"approved"`
}

// SSE event name constants
const (
	SSEToken            = "token"
	SSEToolCall         = "tool_call"
	SSEToolResult       = "tool_result"
	SSEApprovalRequired = "approval_required"
	SSEDone             = "done"
	SSEError            = "error"
)

// SSEEvent is the internal goroutine-communication type for SSE frames
type SSEEvent struct {
	Name string
	Data any
}

// TokenPayload is the SSE data for event: token
type TokenPayload struct {
	Text string `json:"text"`
}

// ToolCallPayload is the SSE data for event: tool_call
type ToolCallPayload struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// ToolResultPayload is the SSE data for event: tool_result
type ToolResultPayload struct {
	Tool   string         `json:"tool"`
	Output map[string]any `json:"output"`
}

// ApprovalRequiredPayload is the SSE data for event: approval_required
type ApprovalRequiredPayload struct {
	RequestID string   `json:"request_id"`
	Tool      string   `json:"tool"`
	Command   string   `json:"command"`
	Args      []string `json:"args,omitempty"`
}

// DonePayload is the SSE data for event: done
type DonePayload struct {
	SessionID string `json:"session_id"`
}

// ErrorPayload is the SSE data for event: error
type ErrorPayload struct {
	Message string `json:"message"`
}
