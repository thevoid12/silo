package core

import (
	"context"
	"testing"

	coremodels "silo/pkg/core/models"
)

func TestNewRunner(t *testing.T) {
	a, err := Build(context.Background(), coremodels.BuildConfig{
		Agent:  coremodels.AgentConfig{Name: "silo"},
		Prov:   coremodels.ProviderConfig{Provider: "gemini", LLMModel: "gemini-2.0-flash"},
		APIKey: "fake-key",
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	sr, err := NewRunner("silo", a)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}
	if sr == nil || sr.Runner == nil || sr.Sessions == nil {
		t.Fatal("expected non-nil SiloRunner with Runner and Sessions")
	}
}
