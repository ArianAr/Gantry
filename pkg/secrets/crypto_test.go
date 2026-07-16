package secrets

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	const key = "test-passphrase-please-change"
	const plain = "minioadmin-secret"
	enc, err := Encrypt(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(enc) {
		t.Fatalf("expected envelope, got %q", enc)
	}
	if enc == plain {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := Decrypt(enc, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
	// wrong key fails
	if _, err := Decrypt(enc, "wrong"); err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestEmptyKeyPassthrough(t *testing.T) {
	got, err := Encrypt("secret", "")
	if err != nil || got != "secret" {
		t.Fatalf("encrypt empty key: %q %v", got, err)
	}
	got, err = Decrypt("secret", "")
	if err != nil || got != "secret" {
		t.Fatalf("decrypt empty key: %q %v", got, err)
	}
}

func TestLegacyPlaintextWithKey(t *testing.T) {
	got, err := Decrypt("legacy-plain", "key")
	if err != nil || got != "legacy-plain" {
		t.Fatalf("legacy: %q %v", got, err)
	}
}

func TestIdempotentEncrypt(t *testing.T) {
	enc, _ := Encrypt("x", "k")
	enc2, err := Encrypt(enc, "k")
	if err != nil || enc2 != enc {
		t.Fatalf("re-encrypt: %q %v", enc2, err)
	}
}
