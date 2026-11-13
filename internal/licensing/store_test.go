package licensing

import (
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
