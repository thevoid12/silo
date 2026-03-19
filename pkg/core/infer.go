package core

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
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

		llm, err := buildModel(ctx, prov, apiKey)
		if err != nil {
			return false, fmt.Errorf("infer approval: build model: %w", err)
		}

		text, err := llmInferText(ctx, llm, prompt)
		if err != nil {
			return false, fmt.Errorf("infer approval: %w", err)
		}
		return strings.HasPrefix(strings.TrimSpace(strings.ToLower(text)), "true"), nil
	}
}

// llmInferText runs a single non-streaming LLM call and returns the first text response.
func llmInferText(ctx context.Context, llm model.LLM, prompt string) (string, error) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: prompt}}},
		},
	}
	for resp, err := range llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp != nil && resp.Content != nil {
			for _, p := range resp.Content.Parts {
				if p != nil && p.Text != "" {
					return p.Text, nil
				}
			}
		}
	}
	return "", fmt.Errorf("empty response from LLM")
}
