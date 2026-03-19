package models

import (
	"context"

	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
)

// AgentConfig holds runtime configuration for the ADK agent
type AgentConfig struct {
	Name             string
	MaxIterations    int
	SystemPromptPath string
	RetryOnError     int
	RetryBackoffMs   int
}

// ProviderConfig holds the LLM provider and model to use
type ProviderConfig struct {
	Provider string // gemini | openai | anthropic | openrouter | custom
	LLMModel string
	BaseURL  string // optional: overrides the provider's built-in endpoint
}

// BuildConfig groups all parameters needed to construct an ADK agent
type BuildConfig struct {
	Agent  AgentConfig
	Prov   ProviderConfig
	APIKey string
	Tools  []tool.Tool
}

// ModelFactory creates a model.LLM for the given model name, API key, and optional base URL
type ModelFactory func(ctx context.Context, modelName, apiKey, baseURL string) (model.LLM, error)

// SiloRunner bundles the ADK runner with its session service so callers can create sessions
type SiloRunner struct {
	Runner   *runner.Runner
	Sessions session.Service
}

