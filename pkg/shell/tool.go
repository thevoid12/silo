package shell

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	approvalmodels "silo/pkg/approval/models"
	siloerrors "silo/pkg/errors"
	shellmodels "silo/pkg/shell/models"
)

var defaultSafeEnvKeys = []string{
	"PATH", "HOME", "USER", "SHELL", "TERM", "LANG", "LC_ALL",
	"TZ", "TMPDIR", "XDG_RUNTIME_DIR",
}

// shellToolDescription instructs the LLM: command is the full shell string, run exactly as typed in a terminal
const shellToolDescription = `Execute a shell command on the user's computer.

Set "command" to the full shell string exactly as you would type it in a terminal.
All shell features work: pipes, redirection, variables, quoting, &&, ;, etc.

Examples:
  "ls -la"
  "echo 'hello, silo' > hello_silo.txt"
  "grep -r 'pattern' . | head -20"
  "mkdir -p foo && echo done > foo/result.txt"
  "cat /etc/os-release"
  "pwd && ls"

Do NOT split the command into separate fields — put the entire command in "command".`

type executor struct {
	policy  *shellPolicy
	sandbox *sandbox
	cfg     shellmodels.ToolConfig
	log     *zap.SugaredLogger
}

// AddAllowed adds a command to the runtime allowlist (implements shellmodels.PermissionsUpdater)
func (e *executor) AddAllowed(cmd string) {
	e.policy.addAllowed(cmd)
}

// NewShellTool creates an ADK FunctionTool that runs shell commands with policy enforcement.
// Returns the tool, a PermissionsUpdater for runtime allowlist changes, and any error.
func NewShellTool(cfg shellmodels.ToolConfig) (tool.Tool, shellmodels.PermissionsUpdater, error) {
	if len(cfg.SafeEnvKeys) == 0 {
		cfg.SafeEnvKeys = defaultSafeEnvKeys
	}

	allowlist := cfg.Allowlist
	if cfg.PermissionsFile != "" {
		saved, err := loadPermissions(cfg.PermissionsFile)
		if err != nil {
			return nil, nil, fmt.Errorf("load permissions file: %w", err)
		}
		allowlist = append(allowlist, saved...)
	}

	log := cfg.Logger
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	e := &executor{
		policy:  newPolicy(allowlist, cfg.Blocklist),
		sandbox: newSandbox(cfg.Exec, log),
		cfg:     cfg,
		log:     log,
	}
	t, err := functiontool.New[shellmodels.ShellArgs, shellmodels.ShellResult](
		functiontool.Config{
			Name:        "shell",
			Description: shellToolDescription,
		},
		e.run,
	)
	if err != nil {
		return nil, nil, err
	}
	return t, e, nil
}

// run executes the shell command after policy and approval enforcement.
// Policy is checked against the first word of the command string.
func (e *executor) run(tc tool.Context, args shellmodels.ShellArgs) (shellmodels.ShellResult, error) {
	if args.Command == "" {
		return shellmodels.ShellResult{}, fmt.Errorf("%w: command must not be empty", siloerrors.ErrCommandBlocked)
	}

	firstWord := firstWordOf(args.Command)
	decision := e.policy.check(firstWord)
	e.log.Infow("shell: policy check", "command", args.Command, "first_word", firstWord, "decision", decision)

	if decision == shellmodels.Deny {
		e.log.Warnw("shell: command blocked by policy", "command", args.Command)
		return shellmodels.ShellResult{}, fmt.Errorf("%w: %q", siloerrors.ErrCommandBlocked, args.Command)
	}

	if decision == shellmodels.RequiresApproval {
		if e.cfg.Approval == nil {
			e.log.Errorw("shell: no approval service configured", "command", args.Command)
			return shellmodels.ShellResult{}, fmt.Errorf("%w: no approval service configured for %q", siloerrors.ErrCommandBlocked, args.Command)
		}
		e.log.Infow("shell: requesting approval", "command", args.Command, "call_id", tc.FunctionCallID())
		approved, err := e.cfg.Approval.Request(tc, approvalmodels.ApprovalRequest{
			ID:      tc.FunctionCallID(),
			Tool:    "shell",
			Command: args.Command,
		})
		if err != nil {
			e.log.Errorw("shell: approval request failed", "command", args.Command, "error", err)
			return shellmodels.ShellResult{}, fmt.Errorf("%w: %q", siloerrors.ErrApprovalTimeout, args.Command)
		}
		e.log.Infow("shell: approval received", "command", args.Command, "approved", approved)
		if !approved {
			return shellmodels.ShellResult{}, fmt.Errorf("%w: %q", siloerrors.ErrCommandBlocked, args.Command)
		}
	}

	e.log.Infow("shell: proceeding to execute", "command", args.Command)
	env := buildSafeEnv(os.Environ(), e.cfg.SafeEnvKeys)
	return e.sandbox.execute(tc, args, env)
}
