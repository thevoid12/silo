package approval

import (
	"context"
	"errors"
	"testing"
	"time"

	approvalmodels "silo/pkg/approval/models"
	siloerrors "silo/pkg/errors"
)

func newTestService(timeout time.Duration) approvalmodels.ApprovalService {
	return New(approvalmodels.ServiceConfig{Timeout: timeout, QueueSize: 4})
}

func TestApprove(t *testing.T) {
	svc := newTestService(2 * time.Second)

	req := approvalmodels.ApprovalRequest{ID: "req-1", Tool: "shell", Command: "git", Args: []string{"diff"}}

	go func() {
		r := <-svc.Requests()
		svc.Respond(r.ID, true) //nolint:errcheck
	}()

	approved, err := svc.Request(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approved=true")
	}
}

func TestDeny(t *testing.T) {
	svc := newTestService(2 * time.Second)

	req := approvalmodels.ApprovalRequest{ID: "req-2", Tool: "shell", Command: "curl"}

	go func() {
		r := <-svc.Requests()
		svc.Respond(r.ID, false) //nolint:errcheck
	}()

	approved, err := svc.Request(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected approved=false")
	}
}

func TestTimeout(t *testing.T) {
	svc := newTestService(50 * time.Millisecond)

	req := approvalmodels.ApprovalRequest{ID: "req-3", Tool: "shell", Command: "wget"}
	go func() { <-svc.Requests() }()

	_, err := svc.Request(context.Background(), req)
	if !errors.Is(err, siloerrors.ErrApprovalTimeout) {
		t.Errorf("expected ErrApprovalTimeout, got %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	svc := newTestService(30 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := approvalmodels.ApprovalRequest{ID: "req-4", Tool: "shell", Command: "ssh"}
	_, err := svc.Request(ctx, req)
	if !errors.Is(err, siloerrors.ErrApprovalTimeout) {
		t.Errorf("expected ErrApprovalTimeout on cancelled ctx, got %v", err)
	}
}

func TestRespondUnknownID(t *testing.T) {
	svc := newTestService(2 * time.Second)

	err := svc.Respond("does-not-exist", true)
	if !errors.Is(err, siloerrors.ErrApprovalNotFound) {
		t.Errorf("expected ErrApprovalNotFound, got %v", err)
	}
}

func TestPending(t *testing.T) {
	svc := newTestService(2 * time.Second)

	req := approvalmodels.ApprovalRequest{ID: "req-5", Tool: "shell", Command: "make"}

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.Request(context.Background(), req) //nolint:errcheck
	}()

	<-svc.Requests()

	pending := svc.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(pending))
	}
	if pending[0].Command != "make" {
		t.Errorf("expected command 'make', got %q", pending[0].Command)
	}

	svc.Respond(pending[0].ID, true) //nolint:errcheck
	<-done
}

func TestMissingID(t *testing.T) {
	svc := newTestService(2 * time.Second)

	req := approvalmodels.ApprovalRequest{Tool: "shell", Command: "ls"} // no ID
	_, err := svc.Request(context.Background(), req)
	if !errors.Is(err, siloerrors.ErrMissingID) {
		t.Errorf("expected ErrMissingID, got %v", err)
	}
}
