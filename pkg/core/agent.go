package core

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"silo/pkg/config"
	coremodels "silo/pkg/core/models"
	siloerrors "silo/pkg/errors"
)

var (
	registryMu    sync.RWMutex
	modelRegistry = map[string]coremodels.ModelFactory{}
)

// I am using factory pattern+ registery pattern here for easy extension and less changes
func init() {
	RegisterModelFactory("gemini", func(ctx context.Context, modelName, apiKey string) (model.LLM, error) {
		return gemini.NewModel(ctx, modelName, &genai.ClientConfig{APIKey: apiKey})
	})
}

// RegisterModelFactory registers a provider factory so Build can resolve it by name
func RegisterModelFactory(provider string, factory coremodels.ModelFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	modelRegistry[provider] = factory
}

// Build creates an ADK LlmAgent from the given BuildConfig
func Build(ctx context.Context, cfg coremodels.BuildConfig) (agent.Agent, error) {
	llm, err := buildModel(ctx, cfg.Prov, cfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", siloerrors.ErrAgentBuild, err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        cfg.Agent.Name,
		Model:       llm,
		Instruction: resolveSystemPrompt(cfg.Agent.SystemPromptPath),
		Tools:       cfg.Tools,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", siloerrors.ErrAgentBuild, err)
	}

	return a, nil
}

// buildModel looks up the registered factory for the provider and creates the model
func buildModel(ctx context.Context, prov coremodels.ProviderConfig, apiKey string) (model.LLM, error) {
	registryMu.RLock()
	factory, ok := modelRegistry[prov.Provider]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %q", siloerrors.ErrUnsupportedProvider, prov.Provider)
	}
	return factory(ctx, prov.LLMModel, apiKey)
}

// resolveSystemPrompt loads from toml file at path if set, falls back to built-in default
func resolveSystemPrompt(path string) string {
	if path == "" {
		return config.DefaultSystemPrompt()
	}
	prompt, err := config.LoadSystemPrompt(path)
	if err != nil || prompt == "" {
		return config.DefaultSystemPrompt()
	}
	return prompt
}
