package core

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"

	"silo/pkg/config"
	anthropicadapter "silo/pkg/core/llm_providers/anthropic"
	openaicompat "silo/pkg/core/llm_providers/openaicompat"
	coremodels "silo/pkg/core/models"
	siloerrors "silo/pkg/errors"
)

var (
	registryMu    sync.RWMutex
	modelRegistry = map[string]coremodels.ModelFactory{}
)

func init() {
	// anthropic uses its own native SDK
	RegisterModelFactory("anthropic", func(_ context.Context, modelName, apiKey, _ string) (model.LLM, error) {
		return anthropicadapter.New(modelName, apiKey), nil
	})

	// everything else (openai, gemini, openrouter, custom) uses OpenAI-compat SDK
	RegisterModelFactory("_openaicompat", func(_ context.Context, modelName, apiKey, baseURL string) (model.LLM, error) {
		return openaicompat.New(modelName, apiKey, baseURL), nil
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

// buildModel resolves the factory for the provider and constructs the model.LLM
func buildModel(ctx context.Context, prov coremodels.ProviderConfig, apiKey string) (model.LLM, error) {
	registryMu.RLock()
	factory, ok := modelRegistry[prov.Provider]
	if !ok {
		factory, ok = modelRegistry["_openaicompat"]
	}
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %q", siloerrors.ErrUnsupportedProvider, prov.Provider)
	}

	baseURL := prov.BaseURL
	if baseURL == "" {
		baseURL = openaicompat.BuiltinBaseURL(prov.Provider)
	}

	return factory(ctx, prov.LLMModel, apiKey, baseURL)
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
