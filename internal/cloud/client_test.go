package cloud

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/adapters/sqlite"
	"github.com/hesora-ir/hesora-free/internal/licensing"
)

func TestServiceURLsRequireTLSAndRejectCredentials(t *testing.T) {
	for _, raw := range []string{"http://hesora.ir", "https://user:pass@hesora.ir", "https://hesora.ir/api", "https://hesora.ir?token=x", "https://hesora.ir#x", "file:///tmp", ""} {
		if _, err := New(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"https://hesora.ir", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if _, err := New(raw); err != nil {
			t.Errorf("rejected %q: %v", raw, err)
		}
	}
}

func TestLoginBackupChecksumAndLogout(t *testing.T) {
	ctx := context.Background()
	body := []byte("backup bytes")
	hash := sha256.Sum256(body)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Hesora-Client") != "1" {
			t.Error("missing client header")
		}
		if r.URL.Path != "/api/v1/auth/login" && r.Header.Get("Authorization") != "Bearer session" {
			t.Error("missing bearer")
		}
		switch r.URL.Path {
		case "/api/v1/auth/login":
			fmt.Fprint(w, `{"token":"session"}`)
		case "/api/v1/backups":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/octet-stream" || r.Header.Get("X-Backup-SHA256") != hex.EncodeToString(hash[:]) {
				t.Error("invalid backup request")
			}
			fmt.Fprint(w, `{"id":"backup-1","name":"backup.db","size":12}`)
		case "/api/v1/auth/logout":
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":"offline"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, _ := New(server.URL)
	if err := c.Login(ctx, "user@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	if b, err := c.UploadBackup(ctx, file); err != nil || b.ID != "backup-1" {
		t.Fatalf("backup=%+v err=%v", b, err)
	}
	if err := c.Logout(ctx); err == nil || c.token != "" {
		t.Fatal("logout must clear local session even offline")
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestRejectRedirectAndExposeConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			http.Redirect(w, r, "https://example.test/steal", 302)
			return
		}
		w.WriteHeader(409)
		fmt.Fprint(w, `{"error":"version conflict"}`)
	}))
	defer server.Close()
	c, _ := New(server.URL)
	if err := c.Login(context.Background(), "a", "b"); err == nil {
		t.Fatal("followed redirect")
	}
	_, err := c.PutDocument(context.Background(), Document{ID: "invoice-1", Kind: "invoice", Data: json.RawMessage(`{}`)})
	if e, ok := err.(*APIError); !ok || e.Status != 409 {
		t.Fatalf("lost conflict status: %v", err)
	}
}

// Run against the real PHP server using tests/http_test.py in hesora-web.
func TestPHPContract(t *testing.T) {
	base := os.Getenv("HESORA_TEST_API_BASE")
	if base == "" {
		t.Skip("set HESORA_TEST_API_BASE for PHP integration")
	}
	ctx := context.Background()
	c, err := New(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, "user@example.test", os.Getenv("HESORA_TEST_PASSWORD")); err != nil {
		t.Fatal(err)
	}
	defer c.Logout(ctx)
	a, err := c.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Licenses) != 1 || !a.CloudEnabled {
		t.Fatalf("account=%+v", a)
	}
	l := a.Licenses[0]
	key, err := base64.RawURLEncoding.DecodeString(os.Getenv("HESORA_TEST_PUBLIC_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	result := (licensing.Verifier{PublicKey: ed25519.PublicKey(key), InstallationID: l.InstallationID, FreeFeatures: licensing.DefaultFreeFeatures()}).Verify(l.Token)
	if !result.HasFeature(licensing.ProFullFeature) {
		t.Fatalf("PHP license rejected by Go: %v", result.Reason)
	}
	if _, err := c.ValidateLicense(ctx, l.Token, l.InstallationID); err != nil {
		t.Fatal(err)
	}
	s, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	file := filepath.Join(t.TempDir(), "snapshot.db")
	if err := s.BackupTo(ctx, file); err != nil {
		t.Fatal(err)
	}
	b, err := c.UploadBackup(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	backups, err := c.Backups(ctx)
	if err != nil || len(backups) == 0 || b.Checksum == "" {
		t.Fatalf("backups=%+v err=%v", backups, err)
	}
	d, err := c.PutDocument(ctx, Document{ID: "go-contract", Kind: "settings", Data: json.RawMessage(`{"locale":"fa"}`)})
	if err != nil || d.Version != 1 {
		t.Fatalf("document=%+v err=%v", d, err)
	}
	docs, err := c.Documents(ctx)
	if err != nil || len(docs) == 0 {
		t.Fatalf("documents=%+v err=%v", docs, err)
	}
}
