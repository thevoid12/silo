package vault

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	siloerrors "silo/pkg/errors"
	"silo/pkg/vault/models"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type vault struct {
	path    string
	key     []byte
	secrets map[string][]byte
	mu      sync.RWMutex
}

// New returns a SecretVault backed by an encrypted file at the given path
func New(path string) models.SecretVault {
	return &vault{
		path:    path,
		secrets: make(map[string][]byte),
	}
}

// Create initializes and encrypts a new vault file with the given password
func (v *vault) Create(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	salt := make([]byte, models.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	v.key = deriveKey(password, salt)
	v.secrets = make(map[string][]byte)

	return persistEncrypted(v, salt)
}

// Unlock reads and decrypts the vault file using the given password
func (v *vault) Open(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	data, err := os.ReadFile(v.path)
	if os.IsNotExist(err) {
		return siloerrors.ErrVaultNotFound
	}
	if err != nil {
		return fmt.Errorf("read vault: %w", err)
	}

	minSize := models.SaltSize + models.NonceSize + chacha20poly1305.Overhead
	if len(data) < minSize {
		return siloerrors.ErrInvalidPassword
	}

	salt := data[:models.SaltSize]
	nonce := data[models.SaltSize : models.SaltSize+models.NonceSize]
	ciphertext := data[models.SaltSize+models.NonceSize:]

	v.key = deriveKey(password, salt)

	aead, err := chacha20poly1305.NewX(v.key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		zeroBytes(v.key)
		v.key = nil
		return siloerrors.ErrInvalidPassword
	}

	if err := json.Unmarshal(plaintext, &v.secrets); err != nil {
		zeroBytes(v.key)
		v.key = nil
		return fmt.Errorf("decode secrets: %w", err)
	}

	return nil
}

// Lock clears the key and secrets from memory
func (v *vault) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()

	zeroBytes(v.key)
	v.key = nil
	v.secrets = make(map[string][]byte)
}

// IsOpen reports whether the vault is currently open (decrypted in memory)
func (v *vault) IsOpen() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.key != nil
}

// Set stores a secret under key and persists the vault
func (v *vault) WriteSecret(key string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.key == nil {
		return siloerrors.ErrVaultLocked
	}

	v.secrets[key] = value

	data, _ := os.ReadFile(v.path)
	salt := data[:models.SaltSize]

	return persistEncrypted(v, salt)
}

// Get retrieves a secret value by key
func (v *vault) ReadSecret(key string) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.key == nil {
		return nil, siloerrors.ErrVaultLocked
	}

	val, ok := v.secrets[key]
	if !ok {
		return nil, siloerrors.ErrKeyNotFound
	}

	result := make([]byte, len(val))
	copy(result, val)
	return result, nil
}

// Delete removes a secret by key and persists the vault
func (v *vault) RemoveSecret(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.key == nil {
		return siloerrors.ErrVaultLocked
	}

	delete(v.secrets, key)

	data, _ := os.ReadFile(v.path)
	salt := data[:models.SaltSize]

	return persistEncrypted(v, salt)
}

// List returns all secret keys stored in the vault
func (v *vault) ListKeys() ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.key == nil {
		return nil, siloerrors.ErrVaultLocked
	}

	keys := make([]string, 0, len(v.secrets))
	for k := range v.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

// Exists reports whether the vault file exists on disk
func (v *vault) Exists() bool {
	_, err := os.Stat(v.path)
	return err == nil
}

// persistEncrypted encrypts vault secrets and writes them to disk
func persistEncrypted(v *vault, salt []byte) error {
	plaintext, err := json.Marshal(v.secrets)
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}

	aead, err := chacha20poly1305.NewX(v.key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	nonce := make([]byte, models.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	output := make([]byte, 0, models.SaltSize+models.NonceSize+len(ciphertext))
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	if err := os.WriteFile(v.path, output, 0600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}

	return nil
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, models.ArgonTime, models.ArgonMemory, models.ArgonThreads, models.KeySize)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
