package shell

import (
	"fmt"
	"os"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	siloerrors "silo/pkg/errors"
	shellmodels "silo/pkg/shell/models"
)

var defaultSafeEnvKeys = []string{
	"PATH", "HOME", "USER", "SHELL", "TERM", "LANG", "LC_ALL",
	"TZ", "TMPDIR", "XDG_RUNTIME_DIR",
}

type executor struct {
	policy  *shellPolicy
	sandbox *sandbox
	cfg     shellmodels.ToolConfig
}

// NewShellTool creates an ADK FunctionTool that runs shell commands with policy enforcement and sandboxing
func NewShellTool(cfg shellmodels.ToolConfig) (tool.Tool, error) {
	if len(cfg.SafeEnvKeys) == 0 {
		cfg.SafeEnvKeys = defaultSafeEnvKeys
	}
	e := &executor{
		policy:  newPolicy(cfg.Allowlist, cfg.Blocklist),
		sandbox: newSandbox(cfg.Exec),
		cfg:     cfg,
	}
	return functiontool.New[shellmodels.ShellArgs, shellmodels.ShellResult](
		functiontool.Config{
			Name:        "shell",
			Description: "Execute a shell command in a sandboxed environment",
			RequireConfirmationProvider: func(args shellmodels.ShellArgs) bool {
				return e.policy.check(args.Command) == shellmodels.RequiresApproval
			},
		},
		e.run,
	)
}

// run executes the shell command after policy enforcement
func (e *executor) run(tc tool.Context, args shellmodels.ShellArgs) (shellmodels.ShellResult, error) {
	if e.policy.check(args.Command) == shellmodels.Deny {
		return shellmodels.ShellResult{}, fmt.Errorf("%w: %q", siloerrors.ErrCommandBlocked, args.Command)
	}
	env := buildSafeEnv(os.Environ(), e.cfg.SafeEnvKeys)
	return e.sandbox.execute(tc, args, env)
}
