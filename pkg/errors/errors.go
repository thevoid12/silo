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

	// Gateway: 200-249
	CodeServerAlreadyRunning = iota + 186 // starts at 200
	CodeServerNotRunning                  // 201
	CodePIDFileRead                       // 202
	CodePIDFileWrite                      // 203
	CodeGatewayTokenMissing               // 204

	// Database: 250-299
	CodeDBOpen    = iota + 232 // starts at 250
	CodeDBMigrate              // 251
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

	// Gateway errors
	ErrServerAlreadyRunning = &SiloError{Code: CodeServerAlreadyRunning, Message: "server is already running: run \"silo stop\" first"}
	ErrServerNotRunning     = &SiloError{Code: CodeServerNotRunning, Message: "server is not running"}
	ErrPIDFileRead          = &SiloError{Code: CodePIDFileRead, Message: "failed to read PID file"}
	ErrPIDFileWrite         = &SiloError{Code: CodePIDFileWrite, Message: "failed to write PID file"}
	ErrGatewayTokenMissing  = &SiloError{Code: CodeGatewayTokenMissing, Message: "gateway token not found in vault: run \"silo init\" to set up your gateway token"}
	ErrAPIKeyMissing        = &SiloError{Code: CodeGatewayTokenMissing + 1, Message: "API key not set: run \"silo init\" to configure your provider"}
	ErrProviderMissing      = &SiloError{Code: CodeGatewayTokenMissing + 2, Message: "provider not set: check gateway.provider in silo.toml or run \"silo init\""}

	// Database errors
	ErrDBOpen    = &SiloError{Code: CodeDBOpen, Message: "failed to open session database — check session.db_path in silo.toml"}
	ErrDBMigrate = &SiloError{Code: CodeDBMigrate, Message: "failed to migrate session database schema"}
)
