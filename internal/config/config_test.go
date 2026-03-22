package config

import (
	"testing"
)

func TestLoad_MissingMasterKey(t *testing.T) {
	t.Setenv("MASTER_KEY", "")
	t.Setenv("ADMIN_API_KEY", "valid-admin-key-here")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_MissingAdminKey(t *testing.T) {
	t.Setenv("MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("ADMIN_API_KEY", "  ")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_OK(t *testing.T) {
	t.Setenv("MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("ADMIN_API_KEY", "secret-admin")
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminAPIKey != "secret-admin" || c.Port != "8080" {
		t.Fatalf("%+v", c)
	}
}
