package shell

import (
	"path/filepath"
	"strings"
	"sync"

	shellmodels "silo/pkg/shell/models"
)

type shellPolicy struct {
	mu        sync.RWMutex
	allowlist map[string]struct{}
	blocklist map[string]struct{}
}

// newPolicy builds a policy from allow/block lists of command names
func newPolicy(allow, block []string) *shellPolicy {
	p := &shellPolicy{
		allowlist: make(map[string]struct{}, len(allow)),
		blocklist: make(map[string]struct{}, len(block)),
	}
	for _, c := range allow {
		p.allowlist[strings.ToLower(c)] = struct{}{}
	}
	for _, c := range block {
		p.blocklist[strings.ToLower(c)] = struct{}{}
	}
	return p
}

// check returns the policy decision for the given command
func (p *shellPolicy) check(cmd string) shellmodels.PolicyDecision {
	base := extractBaseCmd(cmd)
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.blocklist[base]; ok {
		return shellmodels.Deny
	}
	if _, ok := p.allowlist[base]; ok {
		return shellmodels.Allow
	}
	return shellmodels.RequiresApproval
}

// addAllowed inserts a command into the runtime allowlist
func (p *shellPolicy) addAllowed(cmd string) {
	p.mu.Lock()
	p.allowlist[strings.ToLower(cmd)] = struct{}{}
	p.mu.Unlock()
}

// extractBaseCmd returns the lowercase binary name, stripping any path prefix
func extractBaseCmd(cmd string) string {
	return strings.ToLower(filepath.Base(cmd))
}

// firstWordOf returns the first whitespace-delimited token of a shell command string
func firstWordOf(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if idx := strings.IndexAny(cmd, " \t"); idx != -1 {
		return cmd[:idx]
	}
	return cmd
}

// buildSafeEnv filters the current process env to only include the allowed keys
func buildSafeEnv(current []string, safeKeys []string) []string {
	allowed := make(map[string]struct{}, len(safeKeys))
	for _, k := range safeKeys {
		allowed[strings.ToUpper(k)] = struct{}{}
	}
	result := make([]string, 0, len(safeKeys))
	for _, kv := range current {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			if _, ok := allowed[strings.ToUpper(parts[0])]; ok {
				result = append(result, kv)
			}
		}
	}
	return result
}
