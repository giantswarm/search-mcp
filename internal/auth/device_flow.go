package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// DeviceCodeData represents a pending device authorization
type DeviceCodeData struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresAt       time.Time
	Interval        int // Polling interval in seconds

	// Authorization state
	Authorized bool
	Tokens     *TokenData
}

// DeviceFlowStore manages device authorization requests
type DeviceFlowStore struct {
	mu     sync.RWMutex
	codes  map[string]*DeviceCodeData // device_code -> data
	byCodes map[string]string          // user_code -> device_code
}

// NewDeviceFlowStore creates a new device flow store
func NewDeviceFlowStore() *DeviceFlowStore {
	store := &DeviceFlowStore{
		codes:   make(map[string]*DeviceCodeData),
		byCodes: make(map[string]string),
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

// CreateDeviceCode creates a new device authorization request
func (s *DeviceFlowStore) CreateDeviceCode(verificationURI string) (*DeviceCodeData, error) {
	deviceCode, err := generateRandomCode(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate device code: %w", err)
	}

	userCode, err := generateUserCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user code: %w", err)
	}

	data := &DeviceCodeData{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: verificationURI,
		ExpiresAt:       time.Now().Add(10 * time.Minute), // 10 minute expiry
		Interval:        5,                                  // Poll every 5 seconds
		Authorized:      false,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.codes[deviceCode] = data
	s.byCodes[userCode] = deviceCode

	return data, nil
}

// GetByDeviceCode retrieves device data by device code
func (s *DeviceFlowStore) GetByDeviceCode(deviceCode string) (*DeviceCodeData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.codes[deviceCode]
	if !exists {
		return nil, fmt.Errorf("invalid device code")
	}

	if time.Now().After(data.ExpiresAt) {
		return nil, fmt.Errorf("device code expired")
	}

	return data, nil
}

// GetByUserCode retrieves device data by user code
func (s *DeviceFlowStore) GetByUserCode(userCode string) (*DeviceCodeData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deviceCode, exists := s.byCodes[userCode]
	if !exists {
		return nil, fmt.Errorf("invalid user code")
	}

	data, exists := s.codes[deviceCode]
	if !exists {
		return nil, fmt.Errorf("invalid user code")
	}

	if time.Now().After(data.ExpiresAt) {
		return nil, fmt.Errorf("user code expired")
	}

	return data, nil
}

// AuthorizeDevice marks a device as authorized and stores tokens
func (s *DeviceFlowStore) AuthorizeDevice(userCode string, tokens *TokenData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	deviceCode, exists := s.byCodes[userCode]
	if !exists {
		return fmt.Errorf("invalid user code")
	}

	data, exists := s.codes[deviceCode]
	if !exists {
		return fmt.Errorf("invalid user code")
	}

	if time.Now().After(data.ExpiresAt) {
		return fmt.Errorf("user code expired")
	}

	if data.Authorized {
		return fmt.Errorf("device already authorized")
	}

	data.Authorized = true
	data.Tokens = tokens

	return nil
}

// Delete removes a device code from the store
func (s *DeviceFlowStore) Delete(deviceCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if data, exists := s.codes[deviceCode]; exists {
		delete(s.byCodes, data.UserCode)
		delete(s.codes, deviceCode)
	}
}

// cleanup periodically removes expired device codes
func (s *DeviceFlowStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()

		for deviceCode, data := range s.codes {
			if now.After(data.ExpiresAt) {
				delete(s.byCodes, data.UserCode)
				delete(s.codes, deviceCode)
			}
		}

		s.mu.Unlock()
	}
}

// generateRandomCode generates a cryptographically secure random code
func generateRandomCode(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateUserCode generates a user-friendly code (e.g., "ABCD-EFGH")
func generateUserCode() (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Exclude confusing characters
	const codeLength = 8

	b := make([]byte, codeLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	code := make([]byte, codeLength)
	for i, val := range b {
		code[i] = charset[int(val)%len(charset)]
	}

	// Format as XXXX-XXXX for readability
	return fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:])), nil
}

// DeviceFlowManager extends AuthManager with device flow capabilities
type DeviceFlowManager struct {
	AuthManager
	deviceStore *DeviceFlowStore
	serverAddr  string
}

// InitiateDeviceFlow starts a device authorization flow
func (m *DeviceFlowManager) InitiateDeviceFlow(ctx context.Context) (*DeviceCodeData, error) {
	verificationURI := fmt.Sprintf("http://%s/device", m.serverAddr)
	if m.serverAddr[0] == ':' {
		verificationURI = fmt.Sprintf("http://localhost%s/device", m.serverAddr)
	}

	return m.deviceStore.CreateDeviceCode(verificationURI)
}

// PollForToken polls for authorization and returns tokens when available
func (m *DeviceFlowManager) PollForToken(ctx context.Context, deviceCode string) (*TokenData, error) {
	data, err := m.deviceStore.GetByDeviceCode(deviceCode)
	if err != nil {
		return nil, err
	}

	if !data.Authorized {
		return nil, fmt.Errorf("authorization_pending")
	}

	// Authorization complete, clean up and return tokens
	tokens := data.Tokens
	m.deviceStore.Delete(deviceCode)

	return tokens, nil
}

// GetDeviceByUserCode retrieves device data for user authorization
func (m *DeviceFlowManager) GetDeviceByUserCode(userCode string) (*DeviceCodeData, error) {
	return m.deviceStore.GetByUserCode(userCode)
}

// AuthorizeDeviceWithUserCode completes device authorization
func (m *DeviceFlowManager) AuthorizeDeviceWithUserCode(userCode string, tokens *TokenData) error {
	return m.deviceStore.AuthorizeDevice(userCode, tokens)
}
