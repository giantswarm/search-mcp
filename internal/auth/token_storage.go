package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// TokenStorage handles encrypted storage of OAuth tokens
type TokenStorage interface {
	// Store saves tokens with encryption
	Store(ctx context.Context, tokens *TokenData) error

	// Load retrieves and decrypts tokens
	Load(ctx context.Context) (*TokenData, error)

	// Delete removes token file
	Delete() error

	// Exists checks if token file exists
	Exists() bool
}

// FileTokenStorage implements TokenStorage using encrypted file storage
type FileTokenStorage struct {
	filePath      string
	encryptionKey []byte
}

// NewFileTokenStorage creates a new file-based token storage
func NewFileTokenStorage() (*FileTokenStorage, error) {
	// Get storage directory
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Derive encryption key from machine ID
	encryptionKey, err := DeriveEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	return &FileTokenStorage{
		filePath:      filepath.Join(configDir, "tokens.enc"),
		encryptionKey: encryptionKey,
	}, nil
}

// Store saves tokens with encryption
func (s *FileTokenStorage) Store(ctx context.Context, tokens *TokenData) error {
	if tokens == nil {
		return fmt.Errorf("tokens cannot be nil")
	}

	// Serialize tokens to JSON
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	// Encrypt token data
	encrypted, err := Encrypt(data, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt tokens: %w", err)
	}

	// Write to file with restrictive permissions (0600)
	if err := os.WriteFile(s.filePath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	// Verify permissions are correct
	if err := s.verifyPermissions(); err != nil {
		return fmt.Errorf("token file permissions check failed: %w", err)
	}

	return nil
}

// Load retrieves and decrypts tokens
func (s *FileTokenStorage) Load(ctx context.Context) (*TokenData, error) {
	// Check if file exists
	if !s.Exists() {
		return nil, ErrTokenNotFound
	}

	// Read encrypted data
	encrypted, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// Decrypt
	data, err := Decrypt(encrypted, s.encryptionKey)
	if err != nil {
		// Corrupted file - delete it automatically
		_ = s.Delete()
		return nil, fmt.Errorf("token file corrupted (decryption failed), automatically deleted: %w", err)
	}

	// Deserialize
	var tokens TokenData
	if err := json.Unmarshal(data, &tokens); err != nil {
		// Corrupted file - delete it automatically
		_ = s.Delete()
		return nil, fmt.Errorf("token file corrupted (invalid JSON), automatically deleted: %w", err)
	}

	return &tokens, nil
}

// Delete removes token file
func (s *FileTokenStorage) Delete() error {
	if !s.Exists() {
		return nil
	}

	if err := os.Remove(s.filePath); err != nil {
		return fmt.Errorf("failed to delete token file: %w", err)
	}

	return nil
}

// Exists checks if token file exists
func (s *FileTokenStorage) Exists() bool {
	_, err := os.Stat(s.filePath)
	return err == nil
}

// verifyPermissions ensures token file has correct permissions (0600)
func (s *FileTokenStorage) verifyPermissions() error {
	info, err := os.Stat(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to stat token file: %w", err)
	}

	// On Unix-like systems, check permissions
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0600 {
			return fmt.Errorf("token file has incorrect permissions %o, expected 0600", perm)
		}
	}

	return nil
}

// getConfigDir returns the platform-specific config directory for token storage
func getConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "linux":
		// Linux: Use XDG_CONFIG_HOME or fallback to ~/.config
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			configDir = filepath.Join(xdg, "giantswarm")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			configDir = filepath.Join(home, ".config", "giantswarm")
		}

	case "darwin":
		// macOS: Use ~/Library/Application Support
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support", "giantswarm")

	case "windows":
		// Windows: Use %APPDATA%
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configDir = filepath.Join(appData, "giantswarm")

	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return configDir, nil
}
