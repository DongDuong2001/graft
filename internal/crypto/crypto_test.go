package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 bytes hex encoded -> 64 chars
	plaintext := "my_super_secret_api_key_123!"

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("Ciphertext is empty")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if plaintext != decrypted {
		t.Fatalf("Expected %s but got %s", plaintext, decrypted)
	}
}

func TestDecryptInvalid(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Should fail due to garbage hex
	_, err := Decrypt("not-hex", key)
	if err == nil {
		t.Fatal("Expected error decrypting invalid hex")
	}

	// Should fail due to short ciphertext
	_, err = Decrypt("abcdef", key)
	if err == nil {
		t.Fatal("Expected error decrypting short ciphertext")
	}
}
