package core

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	coremodels "silo/pkg/core/models"
)

// NewRunner creates an ADK Runner for the given agent and returns it with its session service
func NewRunner(appName string, a agent.Agent) (*coremodels.SiloRunner, error) {
	svc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          a,
		SessionService: svc,
	})
	if err != nil {
		return nil, err
	}
	return &coremodels.SiloRunner{Runner: r, Sessions: svc}, nil
}
