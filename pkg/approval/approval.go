package approval

import (
	"context"
	"fmt"
	"sync"
	"time"

	approvalmodels "silo/pkg/approval/models"
	siloerrors "silo/pkg/errors"
)

type approvalService struct {
	cfg      approvalmodels.ServiceConfig
	requests chan approvalmodels.ApprovalRequest
	pending  sync.Map // string → *pendingEntry
}

// New creates an ApprovalService with the given config
func New(cfg approvalmodels.ServiceConfig) approvalmodels.ApprovalService {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 16
	}
	return &approvalService{
		cfg:      cfg,
		requests: make(chan approvalmodels.ApprovalRequest, cfg.QueueSize),
	}
}

// Request submits a tool call for user approval and blocks until decided or timeout
// I am using a Request–reply over asynchronous queue message pattern
func (s *approvalService) Request(ctx context.Context, req approvalmodels.ApprovalRequest) (bool, error) {
	if req.ID == "" {
		return false, siloerrors.ErrMissingID
	}
	respCh := make(chan bool, 1)
	s.pending.Store(req.ID, &approvalmodels.PendingEntry{Req: req, Resp: respCh})
	defer s.pending.Delete(req.ID)

	select {
	case s.requests <- req:
	case <-ctx.Done():
		return false, siloerrors.ErrApprovalTimeout
	}

	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case approved := <-respCh:
		return approved, nil
	case <-time.After(timeout):
		return false, fmt.Errorf("%w: %q", siloerrors.ErrApprovalTimeout, req.ID)
	case <-ctx.Done():
		return false, siloerrors.ErrApprovalTimeout
	}
}

// Respond resolves a pending approval by ID
func (s *approvalService) Respond(id string, approved bool) error {
	val, ok := s.pending.Load(id)
	if !ok {
		return fmt.Errorf("%w: %q", siloerrors.ErrApprovalNotFound, id)
	}
	entry := val.(*approvalmodels.PendingEntry)
	select {
	case entry.Resp <- approved:
		return nil
	default:
		return fmt.Errorf("%w: %q", siloerrors.ErrApprovalNotFound, id)
	}
}

// Pending returns all currently awaiting approval requests
func (s *approvalService) Pending() []approvalmodels.ApprovalRequest {
	var out []approvalmodels.ApprovalRequest
	s.pending.Range(func(_, val any) bool {
		out = append(out, val.(*approvalmodels.PendingEntry).Req)
		return true
	})
	return out
}

// Requests returns the channel UI layers listen on for incoming approval requests
func (s *approvalService) Requests() <-chan approvalmodels.ApprovalRequest {
	return s.requests
}

// GetPending returns a single pending request by ID
func (s *approvalService) GetPending(id string) (approvalmodels.ApprovalRequest, error) {
	val, ok := s.pending.Load(id)
	if !ok {
		return approvalmodels.ApprovalRequest{}, fmt.Errorf("%w: %q", siloerrors.ErrApprovalNotFound, id)
	}
	return val.(*approvalmodels.PendingEntry).Req, nil
}
