package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"

	siloerrors "silo/pkg/errors"
	shellmodels "silo/pkg/shell/models"
)

type sandbox struct {
	cfg shellmodels.ExecConfig
	log *zap.SugaredLogger
}

// newSandbox creates a sandbox with the given execution config
func newSandbox(cfg shellmodels.ExecConfig, log *zap.SugaredLogger) *sandbox {
	return &sandbox{cfg: cfg, log: log}
}

// execute runs a command in the configured working directory with timeout and output limits
func (s *sandbox) execute(ctx context.Context, args shellmodels.ShellArgs, env []string) (shellmodels.ShellResult, error) {
	dir := s.cfg.WorkDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return shellmodels.ShellResult{}, fmt.Errorf("%w: %w", siloerrors.ErrSandboxCreate, err)
		}
	}

	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.log.Infow("shell: executing command", "command", args.Command, "workdir", dir, "timeout", timeout)

	cmd := exec.CommandContext(tctx, "sh", "-c", args.Command)
	cmd.Dir = dir
	cmd.Env = env
	if args.Stdin != "" {
		cmd.Stdin = strings.NewReader(args.Stdin)
	}

	maxBytes := s.cfg.MaxOutputBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	outW := &limitedWriter{w: &stdoutBuf, limit: maxBytes}
	errW := &limitedWriter{w: &stderrBuf, limit: maxBytes}
	cmd.Stdout = outW
	cmd.Stderr = errW

	start := time.Now()
	exitCode := 0
	if runErr := cmd.Run(); runErr != nil {
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			s.log.Errorw("shell: command timed out", "command", args.Command, "timeout", timeout, "elapsed", time.Since(start))
			return shellmodels.ShellResult{}, siloerrors.ErrCommandTimeout
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			s.log.Errorw("shell: command failed", "command", args.Command, "error", runErr, "elapsed", time.Since(start))
			return shellmodels.ShellResult{}, fmt.Errorf("%w: %w", siloerrors.ErrSandboxExecute, runErr)
		}
	}

	s.log.Infow("shell: command finished", "command", args.Command, "exit_code", exitCode, "elapsed", time.Since(start),
		"stdout_bytes", stdoutBuf.Len(), "stderr_bytes", stderrBuf.Len(), "truncated", outW.truncated || errW.truncated)

	return shellmodels.ShellResult{
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String(),
		ExitCode:  exitCode,
		Truncated: outW.truncated || errW.truncated,
	}, nil
}

// limitedWriter caps writes at limit bytes, setting truncated on overflow
type limitedWriter struct {
	w         io.Writer
	limit     int
	n         int
	truncated bool
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	orig := len(p)
	remaining := lw.limit - lw.n
	if remaining <= 0 {
		lw.truncated = true
		return orig, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		lw.truncated = true
	}
	n, err := lw.w.Write(p)
	lw.n += n
	if err != nil {
		return n, err
	}
	return orig, nil
}
