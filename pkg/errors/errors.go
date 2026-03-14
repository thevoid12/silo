package errors

import "fmt"

// SiloError carries an error code for observability alongside a human-readable message
type SiloError struct {
	Code    int
	Message string
}

func (e *SiloError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

const (
	// General: shared across all packages
	CodeMissingID = iota // 0

	// Vault: 1–49
	CodeVaultLocked   = iota // 1
	CodeVaultNotFound        // 2
	CodeKeyNotFound          // 3
	CodeInvalidPassword      // 4

	// Agent/Core: 50–99
	CodeUnsupportedProvider = iota + 45 // starts at 50
	CodeAgentBuild
	CodeRunnerBuild

	// Shell: 100–149
	CodeCommandBlocked = iota + 92 // starts at 100
	CodeCommandTimeout
	CodeSandboxCreate
	CodeSandboxExecute
	CodePathBlocked

	// Approval: 150+
	CodeApprovalTimeout  = iota + 137 // starts at 150
	CodeApprovalNotFound              // 151
)

var (
	// General errors
	ErrMissingID = &SiloError{Code: CodeMissingID, Message: "id must not be empty"}

	// Vault errors
	ErrVaultLocked     = &SiloError{Code: CodeVaultLocked, Message: "vault is locked"}
	ErrVaultNotFound   = &SiloError{Code: CodeVaultNotFound, Message: "vault file not found"}
	ErrKeyNotFound     = &SiloError{Code: CodeKeyNotFound, Message: "key not found"}
	ErrInvalidPassword = &SiloError{Code: CodeInvalidPassword, Message: "invalid password"}

	// Agent/Core errors
	ErrUnsupportedProvider = &SiloError{Code: CodeUnsupportedProvider, Message: "unsupported provider"}
	ErrAgentBuild          = &SiloError{Code: CodeAgentBuild, Message: "failed to build agent"}
	ErrRunnerBuild         = &SiloError{Code: CodeRunnerBuild, Message: "failed to build runner"}

	// Shell errors
	ErrCommandBlocked = &SiloError{Code: CodeCommandBlocked, Message: "command is blocked by policy"}
	ErrCommandTimeout = &SiloError{Code: CodeCommandTimeout, Message: "command timed out"}
	ErrSandboxCreate  = &SiloError{Code: CodeSandboxCreate, Message: "failed to create sandbox"}
	ErrSandboxExecute = &SiloError{Code: CodeSandboxExecute, Message: "sandbox execution failed"}
	ErrPathBlocked    = &SiloError{Code: CodePathBlocked, Message: "path is blocked by policy"}

	// Approval errors
	ErrApprovalTimeout  = &SiloError{Code: CodeApprovalTimeout, Message: "approval request timed out — command denied"}
	ErrApprovalNotFound = &SiloError{Code: CodeApprovalNotFound, Message: "no pending approval found for the given id"}
)
