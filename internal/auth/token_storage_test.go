package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileTokenStorage_StoreAndLoad(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test token data
	testToken := &TokenData{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
		IDToken:      "test-id-token",
	}

	// Create storage with test encryption key
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i)
	}

	storage := &FileTokenStorage{
		filePath:      filepath.Join(tempDir, "tokens.enc"),
		encryptionKey: testKey,
	}

	ctx := context.Background()

	// Test Store
	err = storage.Store(ctx, testToken)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify file exists
	if !storage.Exists() {
		t.Error("Token file should exist after Store")
	}

	// Test Load
	loadedToken, err := storage.Load(ctx)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded token matches stored token
	if loadedToken.AccessToken != testToken.AccessToken {
		t.Errorf("AccessToken mismatch: got %s, want %s", loadedToken.AccessToken, testToken.AccessToken)
	}
	if loadedToken.RefreshToken != testToken.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %s, want %s", loadedToken.RefreshToken, testToken.RefreshToken)
	}
	if loadedToken.TokenType != testToken.TokenType {
		t.Errorf("TokenType mismatch: got %s, want %s", loadedToken.TokenType, testToken.TokenType)
	}
	if loadedToken.IDToken != testToken.IDToken {
		t.Errorf("IDToken mismatch: got %s, want %s", loadedToken.IDToken, testToken.IDToken)
	}

	// Verify expiry is close (within 1 second due to serialization)
	if loadedToken.Expiry.Sub(testToken.Expiry).Abs() > time.Second {
		t.Errorf("Expiry mismatch: got %v, want %v", loadedToken.Expiry, testToken.Expiry)
	}
}

func TestFileTokenStorage_LoadNonExistent(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testKey := make([]byte, 32)
	storage := &FileTokenStorage{
		filePath:      filepath.Join(tempDir, "nonexistent.enc"),
		encryptionKey: testKey,
	}

	ctx := context.Background()

	// Try to load non-existent file
	_, err = storage.Load(ctx)
	if err != ErrTokenNotFound {
		t.Errorf("Expected ErrTokenNotFound, got %v", err)
	}
}

func TestFileTokenStorage_Delete(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testToken := &TokenData{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	testKey := make([]byte, 32)
	storage := &FileTokenStorage{
		filePath:      filepath.Join(tempDir, "tokens.enc"),
		encryptionKey: testKey,
	}

	ctx := context.Background()

	// Store token
	err = storage.Store(ctx, testToken)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify exists
	if !storage.Exists() {
		t.Error("Token file should exist after Store")
	}

	// Delete
	err = storage.Delete()
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify doesn't exist
	if storage.Exists() {
		t.Error("Token file should not exist after Delete")
	}
}

func TestFileTokenStorage_DeleteNonExistent(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testKey := make([]byte, 32)
	storage := &FileTokenStorage{
		filePath:      filepath.Join(tempDir, "nonexistent.enc"),
		encryptionKey: testKey,
	}

	// Delete non-existent file should not error
	err = storage.Delete()
	if err != nil {
		t.Errorf("Delete of non-existent file should not error: %v", err)
	}
}

func TestFileTokenStorage_CorruptedFile(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testKey := make([]byte, 32)
	filePath := filepath.Join(tempDir, "tokens.enc")

	storage := &FileTokenStorage{
		filePath:      filePath,
		encryptionKey: testKey,
	}

	// Write corrupted data to file
	err = os.WriteFile(filePath, []byte("corrupted data"), 0600)
	if err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	ctx := context.Background()

	// Try to load corrupted file
	_, err = storage.Load(ctx)
	if err == nil {
		t.Error("Expected error when loading corrupted file")
	}

	// Verify file was automatically deleted
	if storage.Exists() {
		t.Error("Corrupted file should be automatically deleted")
	}
}

func TestFileTokenStorage_StoreNilToken(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testKey := make([]byte, 32)
	storage := &FileTokenStorage{
		filePath:      filepath.Join(tempDir, "tokens.enc"),
		encryptionKey: testKey,
	}

	ctx := context.Background()

	// Try to store nil token
	err = storage.Store(ctx, nil)
	if err == nil {
		t.Error("Expected error when storing nil token")
	}
}

func TestFileTokenStorage_FilePermissions(t *testing.T) {
	// Skip on Windows where permission model is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "token-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testToken := &TokenData{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	testKey := make([]byte, 32)
	filePath := filepath.Join(tempDir, "tokens.enc")

	storage := &FileTokenStorage{
		filePath:      filePath,
		encryptionKey: testKey,
	}

	ctx := context.Background()

	// Store token
	err = storage.Store(ctx, testToken)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("File permissions should be 0600, got %o", perm)
	}
}
