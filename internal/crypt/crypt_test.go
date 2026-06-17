package crypt

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	plaintext := []byte("# 2026-06-16\n\n## 09:14:23 #project\nBody line.\n\n")
	var buf bytes.Buffer
	if err := Encrypt(&buf, "correct horse battery staple", plaintext); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !IsEncrypted(buf.Bytes()) {
		t.Fatalf("IsEncrypted false on freshly encrypted stream")
	}
	out, err := Decrypt(bytes.NewReader(buf.Bytes()), "correct horse battery staple")
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(out, plaintext) {
		t.Errorf("round-trip mismatch:\nwant %q\ngot  %q", plaintext, out)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Encrypt(&buf, "right pass", []byte("hello")); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(bytes.NewReader(buf.Bytes()), "wrong pass"); err == nil {
		t.Fatal("Decrypt with wrong passphrase succeeded")
	}
}

func TestEmptyPassphraseRejected(t *testing.T) {
	t.Parallel()
	if err := Encrypt(&bytes.Buffer{}, "", []byte("x")); err == nil {
		t.Fatal("Encrypt with empty passphrase succeeded")
	}
	if _, err := Decrypt(bytes.NewReader([]byte("x")), ""); err == nil {
		t.Fatal("Decrypt with empty passphrase succeeded")
	}
}

func TestIsEncryptedFalseOnPlaintext(t *testing.T) {
	t.Parallel()
	if IsEncrypted([]byte("# 2026-06-16\n\nbody\n")) {
		t.Error("IsEncrypted true on plaintext")
	}
}
