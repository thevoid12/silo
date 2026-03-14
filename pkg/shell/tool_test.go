package shell

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"silo/pkg/approval"
	approvalmodels "silo/pkg/approval/models"
	siloerrors "silo/pkg/errors"
	shellmodels "silo/pkg/shell/models"
)

func TestExtractBaseCmd(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/usr/bin/ls", "ls"},
		{"ls", "ls"},
		{"/bin/RM", "rm"},
		{"./script.sh", "script.sh"},
	}
	for _, c := range cases {
		got := extractBaseCmd(c.input)
		if got != c.want {
			t.Errorf("extractBaseCmd(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestShellPolicy_check(t *testing.T) {
	p := newPolicy([]string{"ls", "echo"}, []string{"rm", "dd"})

	cases := []struct {
		cmd  string
		want shellmodels.PolicyDecision
	}{
		{"ls", shellmodels.Allow},
		{"/bin/echo", shellmodels.Allow},
		{"rm", shellmodels.Deny},
		{"/bin/dd", shellmodels.Deny},
		{"curl", shellmodels.RequiresApproval},
		{"git", shellmodels.RequiresApproval},
	}
	for _, c := range cases {
		got := p.check(c.cmd)
		if got != c.want {
			t.Errorf("check(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestBuildSafeEnv(t *testing.T) {
	current := []string{
		"PATH=/usr/bin",
		"HOME=/root",
		"AWS_SECRET_ACCESS_KEY=supersecret",
		"GITHUB_TOKEN=tok",
		"TERM=xterm",
	}
	safe := []string{"PATH", "HOME", "TERM"}
	result := buildSafeEnv(current, safe)

	for _, kv := range result {
		if strings.Contains(kv, "AWS_SECRET") || strings.Contains(kv, "GITHUB_TOKEN") {
			t.Errorf("unsafe env var leaked: %s", kv)
		}
	}
	if len(result) != 3 {
		t.Errorf("expected 3 safe vars, got %d: %v", len(result), result)
	}
}

func TestSandbox_execute_success(t *testing.T) {
	s := newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second})
	result, err := s.execute(context.Background(), shellmodels.ShellArgs{
		Command: "echo",
		Args:    []string{"hello"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Errorf("expected stdout 'hello', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestSandbox_execute_nonzeroExit(t *testing.T) {
	s := newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second})
	result, err := s.execute(context.Background(), shellmodels.ShellArgs{
		Command: "sh",
		Args:    []string{"-c", "exit 42"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestSandbox_execute_timeout(t *testing.T) {
	s := newSandbox(shellmodels.ExecConfig{Timeout: 50 * time.Millisecond})
	_, err := s.execute(context.Background(), shellmodels.ShellArgs{
		Command: "sleep",
		Args:    []string{"10"},
	}, nil)
	if !errors.Is(err, siloerrors.ErrCommandTimeout) {
		t.Errorf("expected ErrCommandTimeout, got %v", err)
	}
}

func TestSandbox_execute_outputTruncation(t *testing.T) {
	s := newSandbox(shellmodels.ExecConfig{
		Timeout:        5 * time.Second,
		MaxOutputBytes: 10,
	})
	result, err := s.execute(context.Background(), shellmodels.ShellArgs{
		Command: "sh",
		Args:    []string{"-c", "echo 12345678901234567890"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true")
	}
	if len(result.Stdout) > 10 {
		t.Errorf("stdout should be at most 10 bytes, got %d", len(result.Stdout))
	}
}

func TestNewShellTool_creation(t *testing.T) {
	tool, err := NewShellTool(shellmodels.ToolConfig{
		Allowlist: []string{"echo"},
		Blocklist: []string{"rm"},
	})
	if err != nil {
		t.Fatalf("NewShellTool failed: %v", err)
	}
	if tool.Name() != "shell" {
		t.Errorf("expected tool name 'shell', got %q", tool.Name())
	}
}

func TestExecutor_run_blocked(t *testing.T) {
	e := &executor{
		policy:  newPolicy(nil, []string{"rm"}),
		sandbox: newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second}),
		cfg:     shellmodels.ToolConfig{SafeEnvKeys: defaultSafeEnvKeys},
	}
	_, err := e.run(nil, shellmodels.ShellArgs{Command: "rm", Args: []string{"-rf", "/"}})
	if !errors.Is(err, siloerrors.ErrCommandBlocked) {
		t.Errorf("expected ErrCommandBlocked, got %v", err)
	}
}

func TestExecutor_run_requiresApproval_approved(t *testing.T) {
	svc := approval.New(approvalmodels.ServiceConfig{Timeout: 2 * time.Second})
	e := &executor{
		policy:  newPolicy(nil, nil), // everything requires approval
		sandbox: newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second}),
		cfg:     shellmodels.ToolConfig{SafeEnvKeys: defaultSafeEnvKeys, Approval: svc},
	}

	go func() {
		r := <-svc.Requests()
		svc.Respond(r.ID, true) //nolint:errcheck
	}()

	tc := testToolCtx{context.Background()}
	result, err := e.run(tc, shellmodels.ShellArgs{Command: "echo", Args: []string{"hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "hi" {
		t.Errorf("expected stdout 'hi', got %q", result.Stdout)
	}
}

func TestExecutor_run_requiresApproval_denied(t *testing.T) {
	svc := approval.New(approvalmodels.ServiceConfig{Timeout: 2 * time.Second})
	e := &executor{
		policy:  newPolicy(nil, nil), // everything requires approval
		sandbox: newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second}),
		cfg:     shellmodels.ToolConfig{SafeEnvKeys: defaultSafeEnvKeys, Approval: svc},
	}

	go func() {
		r := <-svc.Requests()
		svc.Respond(r.ID, false) //nolint:errcheck
	}()

	tc := testToolCtx{context.Background()}
	_, err := e.run(tc, shellmodels.ShellArgs{Command: "curl", Args: []string{"http://example.com"}})
	if !errors.Is(err, siloerrors.ErrCommandBlocked) {
		t.Errorf("expected ErrCommandBlocked, got %v", err)
	}
}

func TestExecutor_run_requiresApproval_noService(t *testing.T) {
	e := &executor{
		policy:  newPolicy(nil, nil), // everything requires approval
		sandbox: newSandbox(shellmodels.ExecConfig{Timeout: 5 * time.Second}),
		cfg:     shellmodels.ToolConfig{SafeEnvKeys: defaultSafeEnvKeys, Approval: nil},
	}
	_, err := e.run(nil, shellmodels.ShellArgs{Command: "curl"})
	if !errors.Is(err, siloerrors.ErrCommandBlocked) {
		t.Errorf("expected ErrCommandBlocked when no approval service, got %v", err)
	}
}
