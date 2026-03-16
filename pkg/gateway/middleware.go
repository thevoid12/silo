package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"silo/pkg/logger"
)

// BearerAuth validates the Authorization: Bearer <token> header
func BearerAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// InjectLogger injects l into every request's context so handlers can retrieve it via logFromCtx
func InjectLogger(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.SetLoggerctx(c.Request.Context(), l))
		c.Next()
	}
}

// logFromCtx retrieves the zap sugar logger from ctx; falls back to a no-op logger if not set
func logFromCtx(ctx context.Context) *zap.SugaredLogger {
	l := logger.GetLoggerctx(ctx)
	if l == nil {
		l = zap.NewNop()
	}
	return l.Sugar()
}
