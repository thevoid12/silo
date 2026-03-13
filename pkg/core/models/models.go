package models

import (
	"context"

	"google.golang.org/adk/model"
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
	Provider string // gemini | openai | anthropic | ollama
	LLMModel string
}

// BuildConfig groups all parameters needed to construct an ADK agent
type BuildConfig struct {
	Agent  AgentConfig
	Prov   ProviderConfig
	APIKey string
	Tools  []tool.Tool
}

// ModelFactory creates a model.LLM for the given model name and API key
type ModelFactory func(ctx context.Context, modelName, apiKey string) (model.LLM, error)
