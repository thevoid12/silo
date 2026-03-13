package core

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
)

// NewRunner creates an ADK Runner for the given agent using an in-memory session service
func NewRunner(appName string, a agent.Agent) (*runner.Runner, error) {
	return runner.New(runner.Config{
		AppName:        appName,
		Agent:          a,
		SessionService: session.InMemoryService(),
	})
}
