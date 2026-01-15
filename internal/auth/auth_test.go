package auth

import (
	"testing"
	"time"
)

func TestTokenData_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		token    *TokenData
		expected bool
	}{
		{
			name:     "nil token",
			token:    nil,
			expected: false,
		},
		{
			name: "empty access token",
			token: &TokenData{
				AccessToken: "",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expired token",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(-1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "valid token",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "token expiring soon but still valid",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(5 * time.Minute),
			},
			expected: true,
		},
		{
			name: "token expires exactly now",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now(),
			},
			expected: false, // Already expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestTokenData_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name        string
		token       *TokenData
		expected    bool
		description string
	}{
		{
			name:        "nil token",
			token:       nil,
			expected:    false,
			description: "nil token should not need refresh",
		},
		{
			name: "empty access token",
			token: &TokenData{
				AccessToken: "",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			expected:    false,
			description: "empty token should not need refresh",
		},
		{
			name: "token with plenty of time",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			expected:    false,
			description: "token with 60 minutes remaining should not need refresh",
		},
		{
			name: "token with 15 minutes remaining",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(15 * time.Minute),
			},
			expected:    false,
			description: "token with 15 minutes remaining should not need refresh yet",
		},
		{
			name: "token with 11 minutes remaining",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(11 * time.Minute),
			},
			expected:    true,
			description: "token with <12 minutes remaining should need refresh",
		},
		{
			name: "token with 5 minutes remaining",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(5 * time.Minute),
			},
			expected:    true,
			description: "token with 5 minutes remaining definitely needs refresh",
		},
		{
			name: "already expired token",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(-1 * time.Hour),
			},
			expected:    true,
			description: "expired token returns true (needs refresh, but IsValid would be false)",
		},
		{
			name: "token at exactly 12 minute threshold",
			token: &TokenData{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(12 * time.Minute),
			},
			expected:    true,
			description: "token at exactly 12 minutes should need refresh (< 12 min threshold)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.NeedsRefresh()
			if result != tt.expected {
				t.Errorf("NeedsRefresh() = %v, expected %v: %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestTokenData_RefreshThreshold(t *testing.T) {
	// This test verifies the 20% (12 minute) threshold for 1-hour tokens
	// Assumed token lifetime: 1 hour
	// Refresh threshold: 12 minutes (20% of 60 minutes)
	// Implementation uses: timeUntilExpiry < refreshThreshold

	// Token with exactly 12 minutes + 1 second should not need refresh
	token1 := &TokenData{
		AccessToken: "valid-token",
		Expiry:      time.Now().Add(12*time.Minute + 1*time.Second),
	}
	if token1.NeedsRefresh() {
		t.Error("Token with >12 minutes should not need refresh")
	}

	// Token with exactly 12 minutes should need refresh (< threshold)
	token2 := &TokenData{
		AccessToken: "valid-token",
		Expiry:      time.Now().Add(12 * time.Minute),
	}
	if !token2.NeedsRefresh() {
		t.Error("Token with exactly 12 minutes should need refresh")
	}

	// Token with exactly 12 minutes - 1 second should need refresh
	token3 := &TokenData{
		AccessToken: "valid-token",
		Expiry:      time.Now().Add(12*time.Minute - 1*time.Second),
	}
	if !token3.NeedsRefresh() {
		t.Error("Token with <12 minutes should need refresh")
	}
}
