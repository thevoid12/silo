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
	// Vault: 0–49
	CodeVaultLocked = iota
	CodeVaultNotFound
	CodeKeyNotFound
	CodeInvalidPassword

	// Agent/Core: 50–99
	CodeUnsupportedProvider = iota + 46 // starts at 50
	CodeAgentBuild
	CodeRunnerBuild

	// Shell: 100+
	CodeCommandBlocked = iota + 93 // starts at 100
	CodeCommandTimeout
	CodeSandboxCreate
	CodeSandboxExecute
	CodePathBlocked
)

var (
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
	ErrCommandBlocked  = &SiloError{Code: CodeCommandBlocked, Message: "command is blocked by policy"}
	ErrCommandTimeout  = &SiloError{Code: CodeCommandTimeout, Message: "command timed out"}
	ErrSandboxCreate   = &SiloError{Code: CodeSandboxCreate, Message: "failed to create sandbox"}
	ErrSandboxExecute  = &SiloError{Code: CodeSandboxExecute, Message: "sandbox execution failed"}
	ErrPathBlocked     = &SiloError{Code: CodePathBlocked, Message: "path is blocked by policy"}
)
