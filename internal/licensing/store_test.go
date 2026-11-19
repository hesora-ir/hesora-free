package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreTokenRoundTripAndClear(t *testing.T) {
	store := FileStore{Directory: filepath.Join(t.TempDir(), "license-data")}
	if err := store.SaveToken("payload.signature"); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadToken()
	if err != nil {
		t.Fatal(err)
	}
	if got != "payload.signature" {
		t.Fatalf("got %q", got)
	}
	if err := store.ClearToken(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadToken(); !os.IsNotExist(err) {
		t.Fatalf("expected missing token, got %v", err)
	}
}

func TestFileStorePublicKeyRingRoundTripAndLegacyRead(t *testing.T) {
	store := FileStore{Directory: filepath.Join(t.TempDir(), "keys")}
	oldPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]ed25519.PublicKey{"old-2026": oldPublic, "new-2026": newPublic}
	if err = store.SavePublicKeys(keys); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadPublicKeys()
	if err != nil || len(got) != 2 || !got["old-2026"].Equal(oldPublic) || !got["new-2026"].Equal(newPublic) {
		t.Fatalf("key ring=%v err=%v", got, err)
	}
	if err = store.SavePublicKey(oldPublic); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadPublicKeys()
	if err != nil || len(got) != 1 || !got["legacy"].Equal(oldPublic) {
		t.Fatalf("legacy key=%v err=%v", got, err)
	}
}

func TestFileStoreInstallationIDIsStable(t *testing.T) {
	store := FileStore{Directory: filepath.Join(t.TempDir(), "license-data")}
	first, err := store.InstallationID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.InstallationID()
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("installation ID is not stable: %q / %q", first, second)
	}
}

func TestFileStoreRejectsMultilineToken(t *testing.T) {
	store := FileStore{Directory: t.TempDir()}
	if err := store.SaveToken("token\nextra"); err == nil {
		t.Fatal("expected malformed token error")
	}
}
