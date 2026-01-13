package auth

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Test data
	plaintext := []byte("This is a secret token that needs to be encrypted")

	// Generate a 32-byte key for AES-256
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify ciphertext is different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	// Verify decrypted matches original
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data doesn't match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}
}

func TestEncryptWithInvalidKeySize(t *testing.T) {
	plaintext := []byte("test data")

	tests := []struct {
		name    string
		keySize int
	}{
		{"too short", 16},
		{"too long", 48},
		{"empty", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := Encrypt(plaintext, key)
			if err == nil {
				t.Errorf("Expected error for key size %d, got nil", tt.keySize)
			}
		})
	}
}

func TestDecryptWithInvalidKeySize(t *testing.T) {
	ciphertext := []byte("fake ciphertext")

	tests := []struct {
		name    string
		keySize int
	}{
		{"too short", 16},
		{"too long", 48},
		{"empty", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := Decrypt(ciphertext, key)
			if err == nil {
				t.Errorf("Expected error for key size %d, got nil", tt.keySize)
			}
		})
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	plaintext := []byte("secret data")

	// Correct key
	key1 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
	}

	// Wrong key
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 1)
	}

	// Encrypt with key1
	ciphertext, err := Encrypt(plaintext, key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Try to decrypt with key2
	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Expected decryption to fail with wrong key")
	}
}

func TestDecryptWithTamperedCiphertext(t *testing.T) {
	plaintext := []byte("secret data")
	key := make([]byte, 32)

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with ciphertext
	if len(ciphertext) > 20 {
		ciphertext[20] ^= 0xFF
	}

	// Try to decrypt tampered data
	_, err = Decrypt(ciphertext, key)
	if err == nil {
		t.Error("Expected decryption to fail with tampered ciphertext")
	}
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	plaintext := []byte("")
	key := make([]byte, 32)

	// Encrypt empty data
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt empty data failed: %v", err)
	}

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt empty data failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted empty data doesn't match original")
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	plaintext := []byte("test data")
	key := make([]byte, 32)

	// Encrypt same data multiple times
	ciphertext1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("First encrypt failed: %v", err)
	}

	ciphertext2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v", err)
	}

	// Ciphertexts should be different (due to unique nonces)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Encrypting same data twice should produce different ciphertexts")
	}

	// But both should decrypt to the same plaintext
	decrypted1, _ := Decrypt(ciphertext1, key)
	decrypted2, _ := Decrypt(ciphertext2, key)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Both ciphertexts should decrypt to original plaintext")
	}
}
