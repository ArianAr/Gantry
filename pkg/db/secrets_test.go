package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ArianAr/Gantry/pkg/secrets"
)

func TestProviderSecretEncryption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enc.db")
	const key = "unit-test-secrets-key"
	d, err := Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	p := &Provider{
		Name: "enc", ProviderType: "minio", Region: "us-east-1",
		Endpoint:    "http://127.0.0.1:9000",
		AccessKeyID: "ak", SecretAccessKey: "super-secret-value",
	}
	if err := d.CreateProvider(p); err != nil {
		t.Fatal(err)
	}

	// Raw row in DB should be encrypted
	var raw Provider
	if err := d.Gorm().First(&raw, "id = ?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !secrets.IsEncrypted(raw.SecretAccessKey) {
		t.Fatalf("stored secret not encrypted: %q", raw.SecretAccessKey)
	}
	if strings.Contains(raw.SecretAccessKey, "super-secret-value") {
		t.Fatal("plaintext leaked into stored secret")
	}

	got, err := d.GetProvider(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecretAccessKey != "super-secret-value" {
		t.Fatalf("decrypt got %q", got.SecretAccessKey)
	}
}

func TestMigratePlaintextOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mig.db")
	// Create without key (plaintext)
	d1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{
		Name: "plain", ProviderType: "aws", Region: "us-east-1",
		AccessKeyID: "a", SecretAccessKey: "legacy-secret",
	}
	if err := d1.CreateProvider(p); err != nil {
		t.Fatal(err)
	}
	_ = d1.Close()

	// Reopen with key — should migrate
	d2, err := Open(path, "migrate-key")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d2.Close() })

	var raw Provider
	if err := d2.Gorm().First(&raw, "id = ?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !secrets.IsEncrypted(raw.SecretAccessKey) {
		t.Fatalf("not migrated: %q", raw.SecretAccessKey)
	}
	got, err := d2.GetProvider(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecretAccessKey != "legacy-secret" {
		t.Fatalf("got %q", got.SecretAccessKey)
	}
}
