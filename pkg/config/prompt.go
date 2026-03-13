package config

import (
	"strings"

	"github.com/spf13/viper"
)

const backupSystemPrompt = `You are Silo, a local-first AI assistant. You have access to tools for executing
shell commands, reading files, and writing files.

When you need to perform an action, use the appropriate tool. Always explain what
you are about to do before calling a tool. If a tool call fails, try an alternative
approach or explain what went wrong.

Guidelines:
- Be concise and direct
- Prefer using tools over asking the user to do things manually
- If you are unsure, ask for clarification`

// DefaultSystemPrompt returns the built-in system prompt
func DefaultSystemPrompt() string {
	return backupSystemPrompt
}

// LoadSystemPrompt reads the system prompt from a toml file at path
func LoadSystemPrompt(path string) (string, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return "", err
	}
	return strings.TrimSpace(v.GetString("prompts.default")), nil
}
