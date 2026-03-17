package models

import (
	"time"

	approvalmodels "silo/pkg/approval/models"
	"go.uber.org/zap"
)

// PolicyDecision is the outcome of policy evaluation for a command
type PolicyDecision int

const (
	Allow PolicyDecision = iota
	Deny
	RequiresApproval
)

// ShellArgs is the input schema for the shell tool as seen by the agent.
// Command is the full shell string exactly as you would type it in a terminal.
type ShellArgs struct {
	Command string `json:"command"`          // full shell command, e.g. "ls -la" or "echo 'hi' > file.txt"
	Stdin   string `json:"stdin,omitempty"`  // optional data piped to stdin
}

// ShellResult is what the shell tool returns to the agent
type ShellResult struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Truncated bool   `json:"truncated"`
}

// ExecConfig controls sandbox execution limits
type ExecConfig struct {
	Timeout        time.Duration
	MaxOutputBytes int
	WorkDir        string // Directory to run commands in; defaults to cwd if empty
}

// PermissionsUpdater adds commands to the runtime allowlist
type PermissionsUpdater interface {
	AddAllowed(cmd string)
}

// ToolConfig holds the full shell tool configuration
type ToolConfig struct {
	Allowlist       []string
	Blocklist       []string
	SafeEnvKeys     []string
	Exec            ExecConfig
	Approval        approvalmodels.ApprovalService
	PermissionsFile string // Path to allowed_permissions.md for persistent approvals
	Logger          *zap.SugaredLogger
}
