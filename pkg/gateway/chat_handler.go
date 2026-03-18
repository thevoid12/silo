package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	adkrunner "google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	gatewaymodels "silo/pkg/gateway/models"
)

// handleChat streams agent responses as SSE for POST /silo/brain/chat
func (s *server) handleChat(c *gin.Context) {
	log := logFromCtx(c.Request.Context())

	var req gatewaymodels.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("chat: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Errorw("chat: response writer does not support flushing")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	sessionID := req.SessionID
	if sessionID == "" {
		resp, err := s.deps.Sessions.Create(ctx, &session.CreateRequest{
			AppName:   "silo",
			UserID:    "gateway",
			SessionID: uuid.NewString(),
		})
		if err != nil {
			log.Errorw("chat: failed to create session", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
			return
		}
		sessionID = resp.Session.ID()
	}

	r, err := s.deps.NewRunner(ctx)
	if err != nil {
		log.Errorw("chat: failed to initialize agent runner", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize agent"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	eventCh := make(chan gatewaymodels.SSEEvent, 32)
	runnerDone := make(chan struct{})

	// approval goroutine: forwards pending tool approvals as approval_required SSE events.
	// NOTE: approval service is shared across concurrent sessions; routing is best-effort for MVP.
	go func() {
		for {
			select {
			case pending := <-s.deps.Approval.Requests():
				evt := gatewaymodels.SSEEvent{
					Name: gatewaymodels.SSEApprovalRequired,
					Data: gatewaymodels.ApprovalRequiredPayload{
						RequestID: pending.ID,
						Tool:      pending.Tool,
						Command:   pending.Command,
						Args:      pending.Args,
					},
				}
				select {
				case eventCh <- evt:
				case <-runnerDone:
					return
				case <-ctx.Done():
					return
				}
			case <-runnerDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// runner goroutine: drives ADK and emits SSE events; closes runnerDone when done
	go func() {
		defer close(runnerDone)
		s.streamAgentEvents(ctx, r, req.Message, sessionID, eventCh)
	}()

	for {
		select {
		case evt := <-eventCh:
			writeSSE(c.Writer, evt)
			flusher.Flush()
			if evt.Name == gatewaymodels.SSEDone || evt.Name == gatewaymodels.SSEError {
				if evt.Name == gatewaymodels.SSEError {
					if p, ok := evt.Data.(gatewaymodels.ErrorPayload); ok {
						log.Errorw("chat: agent returned error", "session_id", sessionID, "error", p.Message)
					}
				}
				return
			}
		case <-ctx.Done():
			log.Errorw("chat: context cancelled", "session_id", sessionID, "error", ctx.Err())
			return
		}
	}
}

// streamAgentEvents drives the ADK runner and sends typed SSE events to eventCh
func (s *server) streamAgentEvents(ctx context.Context, r *adkrunner.Runner, message, sessionID string, eventCh chan<- gatewaymodels.SSEEvent) {
	log := logFromCtx(ctx)

	send := func(evt gatewaymodels.SSEEvent) bool {
		select {
		case eventCh <- evt:
			return true
		case <-ctx.Done():
			return false
		}
	}

	msg := genai.NewContentFromText(message, genai.RoleUser)
	for event, err := range r.Run(ctx, "gateway", sessionID, msg, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}) {
		if err != nil {
			log.Errorw("chat: runner error", "session_id", sessionID, "error", err)
			send(gatewaymodels.SSEEvent{Name: gatewaymodels.SSEError, Data: gatewaymodels.ErrorPayload{Message: err.Error()}})
			return
		}
		if event.Content == nil {
			continue
		}

		for _, part := range event.Content.Parts {
			if part.FunctionCall != nil {
				if !send(gatewaymodels.SSEEvent{
					Name: gatewaymodels.SSEToolCall,
					Data: gatewaymodels.ToolCallPayload{Tool: part.FunctionCall.Name, Args: part.FunctionCall.Args},
				}) {
					return
				}
			}
			if part.FunctionResponse != nil {
				if !send(gatewaymodels.SSEEvent{
					Name: gatewaymodels.SSEToolResult,
					Data: gatewaymodels.ToolResultPayload{Tool: part.FunctionResponse.Name, Output: part.FunctionResponse.Response},
				}) {
					return
				}
			}
		}

		if event.IsFinalResponse() {
			send(gatewaymodels.SSEEvent{Name: gatewaymodels.SSEDone, Data: gatewaymodels.DonePayload{SessionID: sessionID}})
			return
		}

		for _, part := range event.Content.Parts {
			if part.Text != "" {
				if !send(gatewaymodels.SSEEvent{Name: gatewaymodels.SSEToken, Data: gatewaymodels.TokenPayload{Text: part.Text}}) {
					return
				}
			}
		}
	}

	send(gatewaymodels.SSEEvent{Name: gatewaymodels.SSEDone, Data: gatewaymodels.DonePayload{SessionID: sessionID}})
}

// writeSSE writes a single SSE frame to w
func writeSSE(w http.ResponseWriter, event gatewaymodels.SSEEvent) {
	data, _ := json.Marshal(event.Data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Name, data)
}
