package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadPermissions reads command names from an allowed_permissions.md file
func loadPermissions(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cmds []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		cmd := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
	}
	return cmds, scanner.Err()
}

// SavePermission appends a command to the allowed_permissions.md file
func SavePermission(path, cmd string) error {
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if os.IsNotExist(statErr) {
		if _, err := fmt.Fprintf(f, "# Silo Allowed Permissions\n\nCommands approved for automatic execution:\n\n"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(f, "- %s\n", cmd)
	return err
}
