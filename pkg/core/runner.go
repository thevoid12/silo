package core

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	coremodels "silo/pkg/core/models"
)

// NewRunner creates an ADK Runner for the given agent.
// If svc is nil, a fresh in-memory session service is created.
func NewRunner(appName string, a agent.Agent, svc session.Service) (*coremodels.SiloRunner, error) {
	if svc == nil {
		svc = session.InMemoryService()
	}
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
