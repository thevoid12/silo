package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"silo/pkg/approval"
	approvalmodels "silo/pkg/approval/models"
	"silo/pkg/config"
	"silo/pkg/core"
	coremodels "silo/pkg/core/models"
	"silo/pkg/shell"
	shellmodels "silo/pkg/shell/models"
	"silo/pkg/vault"
)

var (
	chatModel    string
	chatProvider string
	chatNoTools  bool
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Long: `Start an interactive readline-based chat session in the terminal.

The chat connects directly to the local agent (in-process, no HTTP).
Type your message and press Enter to send. The agent streams responses
token-by-token.

In-session commands:
  /help   - Show available commands
  /clear  - Clear the screen
  /exit   - Exit the chat (same as Ctrl-D)
  /new    - Start a new conversation`,
	RunE: runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatModel, "model", "", "override the default model for this session")
	chatCmd.Flags().StringVar(&chatProvider, "provider", "", "override the default provider for this session")
	chatCmd.Flags().BoolVar(&chatNoTools, "no-tools", false, "disable tool use for this session")
}

func runChat(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vaultPath := viper.GetString("vault.path")
	v := vault.New(vaultPath)
	if !v.Exists() {
		return fmt.Errorf("silo not initialized — run 'silo init' first")
	}

	fmt.Print("Vault password: ")
	pass, err := readPass()
	if err != nil {
		return err
	}
	if err := v.Open(pass); err != nil {
		return fmt.Errorf("failed to unlock vault: %w", err)
	}
	defer v.Close()

	provider := viper.GetString("providers.default")
	if chatProvider != "" {
		provider = chatProvider
	}
	model := viper.GetString(fmt.Sprintf("providers.%s.model", provider))
	if chatModel != "" {
		model = chatModel
	}

	apiKeyBytes, err := v.ReadSecret(fmt.Sprintf("%s_api_key", provider))
	if err != nil {
		return fmt.Errorf("API key not found — run 'silo init' to configure: %w", err)
	}

	approvalTimeout := time.Duration(viper.GetInt("tools.approval.timeout")) * time.Second
	svc := approval.New(approvalmodels.ServiceConfig{Timeout: approvalTimeout})

	agentCfg := coremodels.BuildConfig{
		Agent: coremodels.AgentConfig{
			Name:             "silo",
			SystemPromptPath: viper.GetString("agent.system_prompt_path"),
			MaxIterations:    viper.GetInt("agent.max_iterations"),
		},
		Prov:   coremodels.ProviderConfig{Provider: provider, LLMModel: model},
		APIKey: string(apiKeyBytes),
	}

	permissionsFile := filepath.Join(config.DefaultDataDir(), "allowed_permissions.md")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	var permUpdater shellmodels.PermissionsUpdater
	if !chatNoTools {
		shellTool, updater, err := shell.NewShellTool(shellmodels.ToolConfig{
			Allowlist:       viper.GetStringSlice("tools.shell.allowed_commands"),
			Blocklist:       viper.GetStringSlice("tools.shell.blocked_patterns"),
			Approval:        svc,
			PermissionsFile: permissionsFile,
			Exec: shellmodels.ExecConfig{
				Timeout:        time.Duration(viper.GetInt("tools.shell.timeout_secs")) * time.Second,
				MaxOutputBytes: viper.GetInt("tools.shell.max_output_bytes"),
				WorkDir:        cwd,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create shell tool: %w", err)
		}
		permUpdater = updater
		agentCfg.Tools = append(agentCfg.Tools, shellTool)
	}

	a, err := core.Build(ctx, agentCfg)
	if err != nil {
		return fmt.Errorf("failed to build agent: %w", err)
	}

	siloRunner, err := core.NewRunner("silo", a, nil)
	if err != nil {
		return fmt.Errorf("failed to build runner: %w", err)
	}

	resp, err := siloRunner.Sessions.Create(ctx, &session.CreateRequest{
		AppName:   "silo",
		UserID:    "local",
		SessionID: uuid.New().String(),
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	cs := newChatSession(siloRunner.Runner, svc, resp.Session.ID(), permUpdater, permissionsFile)
	return cs.start(ctx)
}

type chatSession struct {
	runner          *runner.Runner
	approval        approvalmodels.ApprovalService
	sessionID       string
	reader          *bufio.Reader
	permUpdater     shellmodels.PermissionsUpdater
	permissionsFile string
}

// newChatSession initialises a chat session with runner, approval service, session ID, and optional permissions updater
func newChatSession(r *runner.Runner, svc approvalmodels.ApprovalService, sessionID string, updater shellmodels.PermissionsUpdater, permFile string) *chatSession {
	return &chatSession{
		runner:          r,
		approval:        svc,
		sessionID:       sessionID,
		reader:          bufio.NewReader(os.Stdin),
		permUpdater:     updater,
		permissionsFile: permFile,
	}
}

// start runs the interactive readline loop until the user exits
func (c *chatSession) start(ctx context.Context) error {
	fmt.Println("Silo — Type /help for commands, Ctrl-D to exit.")
	fmt.Println()

	go c.handleApprovals(ctx)

	for {
		fmt.Print("you> ")
		line, err := c.reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return nil // EOF / Ctrl-D
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		switch input {
		case "/exit":
			return nil
		case "/help":
			printChatHelp()
			continue
		case "/clear":
			fmt.Print("\033[2J\033[H")
			continue
		case "/new":
			fmt.Println("(new session not supported in V0)")
			continue
		}

		if err := c.send(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
}

// send submits a user message and streams the agent response to stdout.
// SSE mode sends text in streaming chunks AND repeats it in the final event,
// so we track whether we already printed text to avoid duplicating.
func (c *chatSession) send(ctx context.Context, input string) error {
	msg := genai.NewContentFromText(input, genai.RoleUser)

	textPrinted := false
	for event, err := range c.runner.Run(ctx, "local", c.sessionID, msg, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}) {
		if err != nil {
			return err
		}
		if event.Content == nil {
			continue
		}

		for _, part := range event.Content.Parts {
			if part.FunctionCall != nil {
				fmt.Printf("\n[tool: %s] %s\n", part.FunctionCall.Name, formatToolArgs(part.FunctionCall.Args))
				textPrinted = false
			}
		}

		if event.IsFinalResponse() {
			if !textPrinted {
				// No streaming text was printed — print from final response
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						if !textPrinted {
							fmt.Print("silo> ")
						}
						fmt.Print(part.Text)
						textPrinted = true
					}
				}
			}
			fmt.Println()
			textPrinted = false
			continue
		}

		// Streaming event — print text as it arrives
		for _, part := range event.Content.Parts {
			if part.Text != "" {
				if !textPrinted {
					fmt.Print("silo> ")
				}
				fmt.Print(part.Text)
				textPrinted = true
			}
		}
	}
	return nil
}

// handleApprovals listens for pending tool approvals and prompts the user inline
func (c *chatSession) handleApprovals(ctx context.Context) {
	for {
		select {
		case req, ok := <-c.approval.Requests():
			if !ok {
				return
			}
			approved, always := c.promptApproval(req)
			c.approval.Respond(req.ID, approved) //nolint:errcheck
			if approved && always {
				c.rememberPermission(req.Command)
			}
		case <-ctx.Done():
			return
		}
	}
}

// promptApproval asks the user to approve, deny, or always-allow a tool call.
// Returns (approved, always).
func (c *chatSession) promptApproval(req approvalmodels.ApprovalRequest) (bool, bool) {
	fmt.Printf("\n  Allow %q? [y/n/a=always]: ", req.Command)
	line, _ := c.reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	switch answer {
	case "a":
		return true, true
	case "y":
		return true, false
	default:
		return false, false
	}
}

// rememberPermission extracts the binary name from the full command and persists it to the allowlist
func (c *chatSession) rememberPermission(fullCmd string) {
	binary := firstWord(fullCmd)
	if c.permUpdater != nil {
		c.permUpdater.AddAllowed(binary)
	}
	if c.permissionsFile != "" {
		if err := shell.SavePermission(c.permissionsFile, binary); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save permission: %v\n", err)
		}
	}
}

// firstWord returns the first whitespace-delimited token of a string
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, " \t"); idx != -1 {
		return s[:idx]
	}
	return s
}

func printChatHelp() {
	fmt.Println("  /help   — show this message")
	fmt.Println("  /clear  — clear the screen")
	fmt.Println("  /new    — start a new conversation (V1)")
	fmt.Println("  /exit   — exit (same as Ctrl-D)")
}

// formatToolArgs formats function call args for display
func formatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, " ")
}
