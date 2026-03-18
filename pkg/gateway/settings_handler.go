package gateway

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"silo/pkg/config"
	"silo/pkg/gateway/models"
)

func (s *server) handleGetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, models.SettingsResponse{
		Provider:      viper.GetString("providers.default"),
		GeminiModel:   viper.GetString("providers.gemini.model"),
		OpenAIModel:   viper.GetString("providers.openai.model"),
		MaxIterations: viper.GetInt("agent.max_iterations"),
		ApprovalMode:  viper.GetString("tools.approval.mode"),
		ShellTimeout:  viper.GetInt("tools.shell.timeout_secs"),
	})
}

func (s *server) handleUpdateSettings(c *gin.Context) {
	var req models.SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Provider != nil {
		viper.Set("providers.default", *req.Provider)
	}
	if req.GeminiModel != nil {
		viper.Set("providers.gemini.model", *req.GeminiModel)
	}
	if req.OpenAIModel != nil {
		viper.Set("providers.openai.model", *req.OpenAIModel)
	}
	if req.MaxIterations != nil {
		viper.Set("agent.max_iterations", *req.MaxIterations)
	}
	if req.ApprovalMode != nil {
		viper.Set("tools.approval.mode", *req.ApprovalMode)
	}
	if req.ShellTimeout != nil {
		viper.Set("tools.shell.timeout_secs", *req.ShellTimeout)
	}

	if err := persistUserConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save settings: %s", err)})
		return
	}

	s.handleGetSettings(c)
}

// persistUserConfig writes the full merged config to ~/.silo/silo.toml
func persistUserConfig() error {
	path := viper.ConfigFileUsed()
	if path == "" {
		path = config.DefaultConfigPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return viper.WriteConfigAs(path)
}
