package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/adk/model"

	siloerrors "silo/pkg/errors"
	coremodels "silo/pkg/core/models"
)

func TestResolveSystemPrompt_default(t *testing.T) {
	prompt := resolveSystemPrompt("")
	if prompt == "" {
		t.Fatal("expected non-empty default system prompt")
	}
}

func TestResolveSystemPrompt_customFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.toml")
	content := "[prompts]\ndefault = \"custom prompt text\"\n"
	os.WriteFile(path, []byte(content), 0600)

	result := resolveSystemPrompt(path)
	if result != "custom prompt text" {
		t.Fatalf("expected 'custom prompt text', got %q", result)
	}
}

func TestResolveSystemPrompt_missingFile(t *testing.T) {
	prompt := resolveSystemPrompt("/nonexistent/path/prompt.toml")
	if prompt == "" {
		t.Fatal("expected default prompt when file is missing")
	}
}

func TestBuildModel_unsupportedProvider(t *testing.T) {
	_, err := buildModel(context.Background(), coremodels.ProviderConfig{Provider: "openai", LLMModel: "gpt-4o"}, "key")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestBuild_unsupportedProvider(t *testing.T) {
	_, err := Build(context.Background(), coremodels.BuildConfig{
		Agent:  coremodels.AgentConfig{Name: "silo"},
		Prov:   coremodels.ProviderConfig{Provider: "openai", LLMModel: "gpt-4o"},
		APIKey: "key",
	})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestBuild_gemini(t *testing.T) {
	a, err := Build(context.Background(), coremodels.BuildConfig{
		Agent:  coremodels.AgentConfig{Name: "silo"},
		Prov:   coremodels.ProviderConfig{Provider: "gemini", LLMModel: "gemini-2.0-flash"},
		APIKey: "fake-key",
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil agent")
	}
}

// TestRegisterModelFactory_resolvedOnBuild verifies that a registered factory is invoked by Build
func TestRegisterModelFactory_resolvedOnBuild(t *testing.T) {
	sentinelErr := fmt.Errorf("factory was called")
	RegisterModelFactory("custom-provider", func(_ context.Context, _, _ string) (model.LLM, error) {
		return nil, sentinelErr
	})

	_, err := Build(context.Background(), coremodels.BuildConfig{
		Agent:  coremodels.AgentConfig{Name: "silo"},
		Prov:   coremodels.ProviderConfig{Provider: "custom-provider", LLMModel: "some-model"},
		APIKey: "key",
	})

	if errors.Is(err, siloerrors.ErrUnsupportedProvider) {
		t.Fatal("factory not resolved — got ErrUnsupportedProvider instead")
	}
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
}
