package core

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	coremodels "silo/pkg/core/models"
	gatewaymodels "silo/pkg/gateway/models"
)

var approveWords = map[string]bool{
	"yes": true, "y": true, "yep": true, "yeah": true, "yup": true,
	"approve": true, "approved": true, "ok": true, "okay": true,
	"sure": true, "allow": true, "allowed": true, "go": true, "run": true,
	"do it": true, "go ahead": true, "run it": true, "allow it": true,
	"permitted": true, "permit": true, "accepted": true, "accept": true,
}

var denyWords = map[string]bool{
	"no": true, "n": true, "nope": true, "nah": true,
	"deny": true, "denied": true, "cancel": true, "stop": true,
	"block": true, "reject": true, "not allowed": true, "dont": true,
	"don't": true, "do not": true, "skip": true,
}

// BuildApprovalInferrer returns an ApprovalInferFunc that fast-paths common
// approve/deny words and falls back to the LLM for ambiguous responses.
func BuildApprovalInferrer(prov coremodels.ProviderConfig, apiKey string) gatewaymodels.ApprovalInferFunc {
	return func(ctx context.Context, userMessage, toolName, command string) (bool, error) {
		normalized := strings.ToLower(strings.TrimSpace(userMessage))
		normalized = strings.TrimRight(normalized, ".,!")

		if approveWords[normalized] {
			return true, nil
		}
		if denyWords[normalized] {
			return false, nil
		}

		client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
		if err != nil {
			return false, fmt.Errorf("infer approval: create client: %w", err)
		}

		var prompt string
		if toolName != "" || command != "" {
			prompt = fmt.Sprintf(
				"A user was asked to approve or deny running the following tool:\nTool: %s\nCommand: %s\n\nUser responded: %q\n\nReply with only the word \"true\" if the user approved, or \"false\" if they denied.",
				toolName, command, userMessage,
			)
		} else {
			prompt = fmt.Sprintf(
				"A user was asked to approve or deny a tool action. They responded: %q\n\nReply with only the word \"true\" if the user approved, or \"false\" if they denied.",
				userMessage,
			)
		}

		resp, err := client.Models.GenerateContent(ctx, prov.LLMModel, genai.Text(prompt), nil)
		if err != nil {
			return false, fmt.Errorf("infer approval: generate: %w", err)
		}
		if resp == nil || len(resp.Candidates) == 0 {
			return false, fmt.Errorf("infer approval: empty response")
		}

		text := strings.TrimSpace(strings.ToLower(resp.Text()))
		return strings.HasPrefix(text, "true"), nil
	}
}
