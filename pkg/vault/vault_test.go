package vault

import (
	"os"
	"path/filepath"
	"testing"

	siloerrors "silo/pkg/errors"
)

func TestCreateAndUnlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")
	password := "testpassword123"

	v := New(path)

	if err := v.Create(password); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if !v.Exists() {
		t.Fatal("vault file should exist after Create")
	}

	v.Close()

	if err := v.Open(password); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	if !v.IsOpen() {
		t.Fatal("vault should be open after Unlock")
	}
}

func TestUnlockWrongPassword(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")

	v := New(path)

	if err := v.Create("correct"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v.Close()

	err := v.Open("wrong")
	if err != siloerrors.ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestSetGetDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")

	v := New(path)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := v.WriteSecret("api_key", []byte("secret123")); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := v.ReadSecret("api_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "secret123" {
		t.Fatalf("expected 'secret123', got '%s'", val)
	}

	if err := v.RemoveSecret("api_key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = v.ReadSecret("api_key")
	if err != siloerrors.ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")

	v := New(path)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v.WriteSecret("key1", []byte("val1"))
	v.WriteSecret("key2", []byte("val2"))

	keys, err := v.ListKeys()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestLockedOperations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")

	v := New(path)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v.Close()

	if _, err := v.ReadSecret("key"); err != siloerrors.ErrVaultLocked {
		t.Fatalf("expected ErrVaultLocked, got %v", err)
	}

	if err := v.WriteSecret("key", []byte("val")); err != siloerrors.ErrVaultLocked {
		t.Fatalf("expected ErrVaultLocked, got %v", err)
	}

	if _, err := v.ListKeys(); err != siloerrors.ErrVaultLocked {
		t.Fatalf("expected ErrVaultLocked, got %v", err)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vault")

	v := New(path)
	if err := v.Create("password"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	v.WriteSecret("persist_key", []byte("persist_value"))
	v.Close()

	v2 := New(path)
	if err := v2.Open("password"); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	val, err := v2.ReadSecret("persist_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "persist_value" {
		t.Fatalf("expected 'persist_value', got '%s'", val)
	}
}

func TestVaultNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.vault")

	v := New(path)

	err := v.Open("password")
	if err != siloerrors.ErrVaultNotFound {
		t.Fatalf("expected ErrVaultNotFound, got %v", err)
	}
}

func TestExistsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.vault")

	v := New(path)
	if v.Exists() {
		t.Fatal("vault should not exist")
	}
}

func TestCorruptedVault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.vault")

	os.WriteFile(path, []byte("invalid data"), 0600)

	v := New(path)
	err := v.Open("password")
	if err != siloerrors.ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword for corrupted vault, got %v", err)
	}
}
