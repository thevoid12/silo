package models

// Crypto parameters for vault key derivation and encryption
const (
	SaltSize     = 16
	NonceSize    = 24
	KeySize      = 32
	ArgonTime    = 8
	ArgonMemory  = 8 * 1024
	ArgonThreads = 2
)

// SecretVault defines the interface for encrypted secret storage
type SecretVault interface {
	Create(password string) error
	Open(password string) error
	Close()
	IsOpen() bool
	WriteSecret(key string, value []byte) error
	ReadSecret(key string) ([]byte, error)
	RemoveSecret(key string) error
	ListKeys() ([]string, error)
	Exists() bool
}
