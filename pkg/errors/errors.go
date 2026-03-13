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
	// Vault error codes
	CodeVaultLocked = iota
	CodeVaultNotFound
	CodeKeyNotFound
	CodeInvalidPassword
)

var (
	ErrVaultLocked     = &SiloError{Code: CodeVaultLocked, Message: "vault is locked"}
	ErrVaultNotFound   = &SiloError{Code: CodeVaultNotFound, Message: "vault file not found"}
	ErrKeyNotFound     = &SiloError{Code: CodeKeyNotFound, Message: "key not found"}
	ErrInvalidPassword = &SiloError{Code: CodeInvalidPassword, Message: "invalid password"}
)
