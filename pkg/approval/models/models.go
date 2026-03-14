package models

import (
	"context"
	"time"
)

// ApprovalService is the interface for managing tool call approvals across CLI, gateway, and desktop
type ApprovalService interface {
	// Request blocks until the user approves/denies the call or the timeout expires (deny on timeout)
	Request(ctx context.Context, req ApprovalRequest) (bool, error)
	// Respond resolves a pending approval by ID
	Respond(id string, approved bool) error
	// Pending returns all currently awaiting approval requests
	Pending() []ApprovalRequest
	// Requests returns the channel that UI layers listen on for incoming approval requests
	Requests() <-chan ApprovalRequest
}

// ApprovalRequest carries the context for a tool call awaiting user approval
type ApprovalRequest struct {
	ID      string
	Tool    string
	Command string
	Args    []string
}

// ServiceConfig holds the approval service configuration
type ServiceConfig struct {
	Timeout   time.Duration
	QueueSize int
}

type PendingEntry struct {
	Req  ApprovalRequest
	Resp chan bool
}
