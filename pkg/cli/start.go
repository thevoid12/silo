package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	siloerrors "silo/pkg/errors"
	"silo/pkg/gateway"
	gatewaymodels "silo/pkg/gateway/models"
	"silo/pkg/logger"
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

Prompts for your vault password to load the gateway bearer token,
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

// spawnServer spawns a background server process after loading the gateway token from the vault
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

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	args := []string{"start", "--serve"}
	if cfgFile != "" {
		args = append(args, "--config", cfgFile)
	}

	child := exec.Command(exe, args...)
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil
	// Pass token via env; visible only in /proc/<pid>/environ (root-only on Linux)
	child.Env = append(os.Environ(), fmt.Sprintf("SILO_GATEWAY_TOKEN=%s", string(tokenBytes)))

	if err := child.Start(); err != nil {
		return fmt.Errorf("spawn server: %w", err)
	}

	if err := writePID(pidFile, child.Process.Pid); err != nil {
		_ = child.Process.Kill()
		return fmt.Errorf("%w: %s", siloerrors.ErrPIDFileWrite, err)
	}

	// Brief delay to surface immediate failures (port in use, etc.)
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

	host := viper.GetString("gateway.host")
	port := viper.GetInt("gateway.port")
	if host == "" || port == 0 {
		return fmt.Errorf("gateway.host and gateway.port must be set in config")
	}

	readTimeout := parseDurationWithDefault(log, "gateway.timeouts.read", 30*time.Second)
	writeTimeout := parseDurationWithDefault(log, "gateway.timeouts.write", 60*time.Second)
	idleTimeout := parseDurationWithDefault(log, "gateway.timeouts.idle", 120*time.Second)

	cfg := gatewaymodels.ServerConfig{
		Host:         host,
		Port:         port,
		Token:        token,
		PIDFile:      viper.GetString("gateway.pid_file"),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	srv := gateway.New(cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run() }()

	select {
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
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
