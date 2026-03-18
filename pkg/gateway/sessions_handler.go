package gateway

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/adk/session"

	gatewaymodels "silo/pkg/gateway/models"
)

// handleListSessions returns all sessions for the gateway user for GET /silo/vault/sessions
func (s *server) handleListSessions(c *gin.Context) {
	log := logFromCtx(c.Request.Context())

	resp, err := s.deps.Sessions.List(c.Request.Context(), &session.ListRequest{
		AppName: "silo",
		UserID:  "gateway",
	})
	if err != nil {
		log.Errorw("sessions: list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	out := make([]gatewaymodels.SessionResponse, 0, len(resp.Sessions))
	for _, sess := range resp.Sessions {
		out = append(out, gatewaymodels.SessionResponse{
			ID:        sess.ID(),
			AppName:   sess.AppName(),
			UserID:    sess.UserID(),
			UpdatedAt: sess.LastUpdateTime().UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, out)
}
