package gateway

import (
	"net/http"

	"github.com/gin-gonic/gin"

	siloerrors "silo/pkg/errors"
	gatewaymodels "silo/pkg/gateway/models"
)

// handleToolApproval resolves a pending tool call for POST /silo/brain/tool-approval.
// Accepts either approved (bool) or message (natural language inferred by LLM).
func (s *server) handleToolApproval(c *gin.Context) {
	log := logFromCtx(c.Request.Context())

	var req gatewaymodels.ToolApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorw("tool-approval: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	approved := req.Approved
	if req.Message != "" {
		inferred, err := s.deps.InferApproval(c.Request.Context(), req.Message, "", "")
		if err != nil {
			log.Errorw("tool-approval: inference failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to infer approval"})
			return
		}
		approved = inferred
		log.Infow("tool-approval: inferred from message", "request_id", req.RequestID, "message", req.Message, "inferred", approved)
	}

	if err := s.deps.Approval.Respond(req.RequestID, approved); err != nil {
		log.Errorw("tool-approval: request not found", "request_id", req.RequestID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": siloerrors.ErrApprovalNotFound.Message})
		return
	}

	log.Infow("tool-approval: resolved", "request_id", req.RequestID, "approved", approved)
	c.Status(http.StatusOK)
}
