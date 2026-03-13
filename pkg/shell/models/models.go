package models

import (
	"time"
)

// PolicyDecision is the outcome of policy evaluation for a command
type PolicyDecision int

const (
	Allow PolicyDecision = iota
	Deny
	RequiresApproval
)

// ShellArgs is the input schema for the shell tool as seen by the agent
type ShellArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Stdin   string   `json:"stdin,omitempty"`
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
}

// ToolConfig holds the full shell tool configuration
type ToolConfig struct {
	Allowlist   []string
	Blocklist   []string
	SafeEnvKeys []string
	Exec        ExecConfig
}
