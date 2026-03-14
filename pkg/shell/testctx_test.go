package shell

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// testToolCtx is a minimal tool.Context stub for unit tests
type testToolCtx struct {
	context.Context
}

func (t testToolCtx) FunctionCallID() string                                           { return "test-call-id" }
func (t testToolCtx) Actions() *session.EventActions                                   { return nil }
func (t testToolCtx) SearchMemory(_ context.Context, _ string) (*memory.SearchResponse, error) {
	return nil, nil
}
func (t testToolCtx) ToolConfirmation() *toolconfirmation.ToolConfirmation { return nil }
func (t testToolCtx) RequestConfirmation(_ string, _ any) error            { return nil }
func (t testToolCtx) UserContent() *genai.Content                          { return nil }
func (t testToolCtx) InvocationID() string                                 { return "" }
func (t testToolCtx) AgentName() string                                    { return "" }
func (t testToolCtx) ReadonlyState() session.ReadonlyState                 { return nil }
func (t testToolCtx) UserID() string                                       { return "" }
func (t testToolCtx) AppName() string                                      { return "" }
func (t testToolCtx) SessionID() string                                    { return "" }
func (t testToolCtx) Branch() string                                       { return "" }
func (t testToolCtx) Artifacts() agent.Artifacts                           { return nil }
func (t testToolCtx) State() session.State                                 { return nil }
