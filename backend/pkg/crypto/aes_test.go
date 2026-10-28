package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInitKeyFromDataDirLoadsPersistedKey(t *testing.T) {
	t.Setenv(EnvSecretKey, "")
	dataDir := t.TempDir()
	firstKey := []byte("0123456789abcdef0123456789abcdef")
	keyPath := filepath.Join(dataDir, "secret.key")
	if err := os.WriteFile(keyPath, firstKey, 0600); err != nil {
		t.Fatalf("write persisted key: %v", err)
	}

	if err := InitKeyFromDataDir(dataDir); err != nil {
		t.Fatalf("InitKeyFromDataDir: %v", err)
	}
	ciphertext, err := Encrypt("stored secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	clearKey()
	if err := InitKeyFromDataDir(dataDir); err != nil {
		t.Fatalf("InitKeyFromDataDir restart: %v", err)
	}
	if !bytes.Equal(globalKey, firstKey) {
		t.Fatal("restart did not reuse the persisted encryption key")
	}
	plaintext, err := Decrypt(ciphertext)
	if err != nil || plaintext != "stored secret" {
		t.Fatalf("Decrypt = %q, %v; want stored secret", plaintext, err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(keyPath)
		if err != nil {
			t.Fatalf("stat key file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("key file mode = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestInitKeyFromDataDirWithoutKeyDisablesEncryptionForExistingDatabase(t *testing.T) {
	t.Setenv(EnvSecretKey, "")
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "netpanel.db"), []byte("existing"), 0600); err != nil {
		t.Fatalf("create existing database marker: %v", err)
	}
	if err := setKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("set prior in-memory key: %v", err)
	}

	if err := InitKeyFromDataDir(dataDir); err != nil {
		t.Fatalf("InitKeyFromDataDir without key: %v", err)
	}
	if IsInitialized() {
		t.Fatal("encryption remained enabled without a configured key")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "secret.key")); !os.IsNotExist(err) {
		t.Fatalf("plaintext mode unexpectedly created a key file: stat error = %v", err)
	}
}

func TestInitKeyFromDataDirRejectsInvalidConfiguredKey(t *testing.T) {
	t.Setenv(EnvSecretKey, "too-short")
	if err := InitKeyFromDataDir(t.TempDir()); err == nil {
		t.Fatal("InitKeyFromDataDir accepted an invalid configured key")
	}
	if IsInitialized() {
		t.Fatal("invalid key left crypto initialized")
	}
}

func TestEncryptIfKeyAvailableFailsClosed(t *testing.T) {
	if err := setKey([]byte("invalid")); err == nil {
		t.Fatal("setKey accepted an invalid key")
	}
	if _, err := EncryptIfKeyAvailable("secret"); err == nil {
		t.Fatal("encryption helper silently returned plaintext without an initialized key")
	}
}
