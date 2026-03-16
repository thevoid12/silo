package gateway

import (
	"net/http"

	"github.com/gin-gonic/gin"

	siloerrors "silo/pkg/errors"
	gatewaymodels "silo/pkg/gateway/models"
)

// handleToolApproval resolves a pending tool call for POST /silo/brain/tool-approval
func (s *server) handleToolApproval(c *gin.Context) {
	log := logFromCtx(c.Request.Context())

	var req gatewaymodels.ToolApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("tool-approval: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.deps.Approval.Respond(req.RequestID, req.Approved); err != nil {
		log.Errorw("tool-approval: request not found", "request_id", req.RequestID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": siloerrors.ErrApprovalNotFound.Message})
		return
	}

	log.Infow("tool-approval: resolved", "request_id", req.RequestID, "approved", req.Approved)
	c.Status(http.StatusOK)
}
