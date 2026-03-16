package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	adkrunner "google.golang.org/adk/runner"

	"silo/pkg/approval"
	approvalmodels "silo/pkg/approval/models"
	"silo/pkg/config"
	"silo/pkg/core"
	coremodels "silo/pkg/core/models"
	siloerrors "silo/pkg/errors"
	"silo/pkg/gateway"
	gatewaymodels "silo/pkg/gateway/models"
	"silo/pkg/logger"
	"silo/pkg/shell"
	shellmodels "silo/pkg/shell/models"
	"silo/pkg/vault"
)

var (
	headless    bool
	startPort   int
	serveMode   bool
	desktopMode bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Silo server in background",
	Long: `Start the Silo HTTP gateway server in the background.

Prompts for your vault password to load the gateway bearer token and API key,
then spawns the server as a detached background process.

Stop with: silo stop
Check status with: silo status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if serveMode {
			return runServe()
		}
		return spawnServer()
	},
}

// spawnServer reads credentials from the vault and spawns a detached server process
func spawnServer() error {
	pidFile := viper.GetString("gateway.pid_file")

	if pid, err := readPID(pidFile); err == nil && isProcessRunning(pid) {
		return fmt.Errorf("%w (PID %d)", siloerrors.ErrServerAlreadyRunning, pid)
	}

	vaultPath := viper.GetString("vault.path")
	v := vault.New(vaultPath)
	if !v.Exists() {
		return fmt.Errorf("vault not found: run \"silo init\" first")
	}

	fmt.Print("Vault password: ")
	password, err := readPassword()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	if err := v.Open(string(password)); err != nil {
		return err
	}
	defer v.Close()

	tokenBytes, err := v.ReadSecret("gateway-token")
	if err != nil {
		return siloerrors.ErrGatewayTokenMissing
	}

	provider := viper.GetString("providers.default")
	apiKeyBytes, err := v.ReadSecret(fmt.Sprintf("%s_api_key", provider))
	if err != nil {
		return siloerrors.ErrAPIKeyMissing
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	spawnArgs := []string{"start", "--serve"}
	if cfgFile != "" {
		spawnArgs = append(spawnArgs, "--config", cfgFile)
	}

	child := exec.Command(exe, spawnArgs...)
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil
	// Credentials passed via env; readable only in /proc/<pid>/environ (owner/root on Linux)
	child.Env = append(os.Environ(),
		fmt.Sprintf("SILO_GATEWAY_TOKEN=%s", string(tokenBytes)),
		fmt.Sprintf("SILO_API_KEY=%s", string(apiKeyBytes)),
		fmt.Sprintf("SILO_PROVIDER=%s", provider),
	)

	if err := child.Start(); err != nil {
		return fmt.Errorf("spawn server: %w", err)
	}

	if err := writePID(pidFile, child.Process.Pid); err != nil {
		_ = child.Process.Kill()
		return fmt.Errorf("%w: %s", siloerrors.ErrPIDFileWrite, err)
	}

	time.Sleep(250 * time.Millisecond)
	if !isProcessRunning(child.Process.Pid) {
		_ = os.Remove(pidFile)
		return fmt.Errorf("server exited immediately — check port %d is free", viper.GetInt("gateway.port"))
	}

	host := viper.GetString("gateway.host")
	port := viper.GetInt("gateway.port")
	fmt.Printf("silo server started (PID %d) listening on %s:%d\n", child.Process.Pid, host, port)
	return nil
}

// runServe is the internal entry point for the detached server process
func runServe() error {
	log, err := logger.InitializeLogger()
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer log.Sync()

	token := os.Getenv("SILO_GATEWAY_TOKEN")
	if token == "" {
		return siloerrors.ErrGatewayTokenMissing
	}

	apiKey := os.Getenv("SILO_API_KEY")
	if apiKey == "" {
		return siloerrors.ErrAPIKeyMissing
	}

	provider := os.Getenv("SILO_PROVIDER")
	if provider == "" {
		provider = viper.GetString("providers.default")
	}
	if provider == "" {
		return siloerrors.ErrProviderMissing
	}

	host := viper.GetString("gateway.host")
	port := viper.GetInt("gateway.port")
	if host == "" || port == 0 {
		return fmt.Errorf("gateway.host and gateway.port must be set in config")
	}

	llmModel := viper.GetString(fmt.Sprintf("providers.%s.model", provider))
	approvalTimeout := time.Duration(viper.GetInt("tools.approval.timeout")) * time.Second
	shellTimeout := time.Duration(viper.GetInt("tools.shell.timeout_secs")) * time.Second
	homeDir, _ := os.UserHomeDir()
	permissionsFile := filepath.Join(config.DefaultDataDir(), "allowed_permissions.md")

	approvalSvc := approval.New(approvalmodels.ServiceConfig{Timeout: approvalTimeout})

	dbPath := viper.GetString("session.db_path")
	if dbPath == "" {
		return fmt.Errorf("session.db_path must be set in config")
	}
	sharedSessions, err := core.NewSQLiteSessionService(coremodels.SessionConfig{DBPath: dbPath})
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}

	sysPromptPath := viper.GetString("agent.system_prompt_path")
	if sysPromptPath == "" {
		return fmt.Errorf("system prompt path must be set in config")

	}
	agentCoreCfg := coremodels.BuildConfig{
		Agent: coremodels.AgentConfig{
			Name:             "silo",
			SystemPromptPath: sysPromptPath,
			MaxIterations:    viper.GetInt("agent.max_iterations"),
		},
		Prov:   coremodels.ProviderConfig{Provider: provider, LLMModel: llmModel},
		APIKey: apiKey,
	}

	// runnerFactory creates a fresh ADK runner per chat request with the shared approval service
	runnerFactory := func(ctx context.Context) (*adkrunner.Runner, error) {
		shellTool, _, err := shell.NewShellTool(shellmodels.ToolConfig{
			Allowlist:       viper.GetStringSlice("tools.shell.allowed_commands"),
			Blocklist:       viper.GetStringSlice("tools.shell.blocked_patterns"),
			Approval:        approvalSvc,
			PermissionsFile: permissionsFile,
			Exec: shellmodels.ExecConfig{
				Timeout:        shellTimeout,
				MaxOutputBytes: viper.GetInt("tools.shell.max_output_bytes"),
				WorkDir:        homeDir,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("create shell tool: %w", err)
		}
		cfg := agentCoreCfg
		cfg.Tools = append(cfg.Tools, shellTool)
		a, err := core.Build(ctx, cfg)
		if err != nil {
			return nil, err
		}
		sr, err := core.NewRunner("silo", a, sharedSessions)
		if err != nil {
			return nil, err
		}
		return sr.Runner, nil
	}

	deps := gatewaymodels.ServerDeps{
		Sessions:  sharedSessions,
		Approval:  approvalSvc,
		NewRunner: runnerFactory,
	}

	readTimeout := parseDurationWithDefault(log, "gateway.timeouts.read", 30*time.Second)
	idleTimeout := parseDurationWithDefault(log, "gateway.timeouts.idle", 120*time.Second)

	cfg := gatewaymodels.ServerConfig{
		Host:        host,
		Port:        port,
		Token:       token,
		PIDFile:     viper.GetString("gateway.pid_file"),
		ReadTimeout: readTimeout,
		IdleTimeout: idleTimeout,
	}

	srv := gateway.New(cfg, deps, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run() }()

	select {
	case <-quit:
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func init() {
	startCmd.Flags().BoolVar(&headless, "headless", false, "start in headless mode (HTTP server only, deprecated: default behaviour)")
	startCmd.Flags().IntVar(&startPort, "port", 0, "override gateway port")
	startCmd.Flags().BoolVar(&serveMode, "serve", false, "run server directly (internal, used by background spawn)")
	startCmd.Flags().BoolVar(&desktopMode, "desktop-mode", false, "internal flag for desktop app integration")
	startCmd.Flags().MarkHidden("serve")
	startCmd.Flags().MarkHidden("desktop-mode")
}
