package cloud

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestMagicLinkAndSupportTicketUseAuthenticatedAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-session" {
			t.Errorf("missing authorization for %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/v1/account/magic-link":
			if r.Method != http.MethodPost {
				t.Errorf("method=%s", r.Method)
			}
			fmt.Fprintf(w, `{"url":%q}`, "http://"+r.Host+"/magic/abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
		case "/api/v1/support/tickets":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["subject"] != "Help" || body["body"] != "Please help" {
				t.Errorf("body=%v", body)
			}
			fmt.Fprint(w, `{"id":"ticket-123"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.token = "test-session"
	link, err := c.MagicLink(context.Background())
	if err != nil || !strings.HasPrefix(link, server.URL+"/magic/") {
		t.Fatalf("link=%q err=%v", link, err)
	}
	id, err := c.CreateSupportTicket(context.Background(), "Help", "Please help")
	if err != nil || id != "ticket-123" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestTrialStartAndServerValidation(t *testing.T) {
	const installation = "0123456789abcdef0123456789abcdef"
	const fingerprint = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	const token = "signed-trial-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch r.URL.Path {
		case "/api/v1/licenses/trial":
			if body["installation_id"] != installation || body["device_fingerprint"] != fingerprint {
				t.Errorf("trial request=%v", body)
			}
			fmt.Fprintf(w, `{"id":"trial-id","token":%q,"expires_at":2000,"server_time":1000,"public_keys":{}}`, token)
		case "/api/v1/licenses/trial/validate":
			if body["installation_id"] != installation || body["device_fingerprint"] != fingerprint || body["token"] != token {
				t.Errorf("validation request=%v", body)
			}
			fmt.Fprint(w, `{"expires_at":2000,"server_time":1500}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := c.StartTrial(context.Background(), installation, fingerprint)
	if err != nil || grant.Token != token || grant.ServerTime != 1000 {
		t.Fatalf("grant=%+v err=%v", grant, err)
	}
	status, err := c.ValidateTrial(context.Background(), installation, fingerprint, token)
	if err != nil || status.ExpiresAt != 2000 || status.ServerTime != 1500 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestImportBackupToWorkspaceEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/backups/0123456789abcdef0123456789abcdef/import" || r.Header.Get("Authorization") != "Bearer session" {
			t.Errorf("unexpected import request: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"customers":2,"issuers":1,"invoices":3}`)
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	if err := c.ImportBackupToWorkspace(context.Background(), "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
}

func TestShareInvoiceEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/workspace/invoices/share" || r.Header.Get("Authorization") != "Bearer session" {
			t.Errorf("unexpected share request: %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"url":"https://hesora.ir/share/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`)
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	link, err := c.ShareInvoice(context.Background(), map[string]any{"Number": "FACT-1", "Status": "issued"})
	if err != nil || !strings.Contains(link, "/share/") {
		t.Fatalf("share link=%q error=%v", link, err)
	}
}

func TestWorkspacePaymentSyncEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer session" {
			t.Error("missing bearer")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspace/payments":
			var body WorkspacePayment
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ID != strings.Repeat("a", 32) || len(body.Allocations) != 1 || body.Allocations[0].DocumentID != strings.Repeat("b", 32) {
				t.Errorf("unexpected payment payload: %+v", body)
			}
			fmt.Fprint(w, `{"idempotent":true}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspace/payments":
			fmt.Fprint(w, `{"payments":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","amount":100,"currency":"IRT","paid_at":1790000000,"allocations":[{"document_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","number":"FACT-1","amount":100}]}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspace/payments/"+strings.Repeat("a", 32)+"/reverse":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["reason"] != "correction" {
				t.Errorf("unexpected reversal body: %#v", body)
			}
			fmt.Fprint(w, `{"ok":true}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	payment := WorkspacePayment{ID: strings.Repeat("a", 32), Amount: 100, Currency: "IRT", PaidAt: 1790000000, Allocations: []WorkspacePaymentAllocation{{DocumentID: strings.Repeat("b", 32), Amount: 100}}}
	idempotent, err := c.SaveWorkspacePayment(context.Background(), payment)
	if err != nil || !idempotent {
		t.Fatalf("save idempotent=%v err=%v", idempotent, err)
	}
	payments, err := c.WorkspacePayments(context.Background())
	if err != nil || len(payments) != 1 || payments[0].Allocations[0].Number != "FACT-1" {
		t.Fatalf("payments=%+v err=%v", payments, err)
	}
	if err = c.ReverseWorkspacePayment(context.Background(), payment.ID, "correction"); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentsSinceFollowsIncrementalCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer session" {
			t.Error("missing bearer")
		}
		switch r.URL.Query().Get("after") {
		case "7":
			fmt.Fprint(w, `{"changes":[{"id":"doc-a","kind":"customer","version":2,"updated_at":10,"change_cursor":9,"deleted":false,"data":{}}],"cursor":9,"has_more":true}`)
		case "9":
			fmt.Fprint(w, `{"changes":[{"id":"doc-b","kind":"invoice","version":1,"updated_at":11,"change_cursor":12,"deleted":true,"data":{}}],"cursor":12,"has_more":false}`)
		default:
			t.Errorf("unexpected cursor: %s", r.URL.Query().Get("after"))
		}
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	docs, cursor, err := c.DocumentsSince(context.Background(), 7)
	if err != nil || cursor != 12 || len(docs) != 2 || docs[0].ChangeCursor != 9 || !docs[1].Deleted {
		t.Fatalf("docs=%+v cursor=%d err=%v", docs, cursor, err)
	}
}

func TestDesktopWinsRetriesVersionConflictOnce(t *testing.T) {
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			puts++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if puts == 1 {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"error":"stale"}`)
				return
			}
			if body["version"] != float64(2) {
				t.Errorf("retry version=%v", body["version"])
			}
			fmt.Fprint(w, `{"id":"doc-a","kind":"customer","version":3,"updated_at":20,"deleted":false,"data":{"Name":"desktop"}}`)
		case http.MethodGet:
			fmt.Fprint(w, `{"id":"doc-a","kind":"customer","version":2,"updated_at":19,"deleted":false,"data":{"Name":"web"}}`)
		default:
			t.Errorf("unexpected request %s", r.Method)
		}
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	doc, err := c.PutDocumentDesktopWins(context.Background(), Document{ID: "doc-a", Kind: "customer", Version: 1, Data: json.RawMessage(`{"Name":"desktop"}`)})
	if err != nil || doc.Version != 3 || puts != 2 {
		t.Fatalf("doc=%+v puts=%d err=%v", doc, puts, err)
	}
}

func TestPutDocumentThreeWayMergesIndependentFieldsBeforeRetry(t *testing.T) {
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			puts++
			var body struct {
				Version int64          `json:"version"`
				Data    map[string]any `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if puts == 1 {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"error":"stale"}`)
				return
			}
			if body.Version != 4 || body.Data["Name"] != "Desktop" || body.Data["Phone"] != "Web phone" {
				t.Errorf("retry payload version=%d data=%v", body.Version, body.Data)
			}
			fmt.Fprint(w, `{"id":"doc-a","kind":"customer","version":5,"updated_at":30,"deleted":false,"data":{"Name":"Desktop","Phone":"Web phone"}}`)
		case http.MethodGet:
			fmt.Fprint(w, `{"id":"doc-a","kind":"customer","version":4,"updated_at":29,"deleted":false,"data":{"Name":"Web","Phone":"Web phone"}}`)
		default:
			t.Errorf("unexpected request %s", r.Method)
		}
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	base := json.RawMessage(`{"Name":"Base","Phone":"Base phone"}`)
	local := json.RawMessage(`{"Name":"Desktop","Phone":"Base phone"}`)
	remote, conflicts, err := c.PutDocumentThreeWay(context.Background(), Document{ID: "doc-a", Kind: "customer", Version: 3, Data: local}, base, true)
	if err != nil || remote.Version != 5 || puts != 2 || !reflect.DeepEqual(conflicts, []string{"Name"}) {
		t.Fatalf("remote=%+v conflicts=%v puts=%d err=%v", remote, conflicts, puts, err)
	}
}

func TestAccountIncludesStableCloudUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/me" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"user":{"id":"0123456789abcdef0123456789abcdef","name":"User","email":"user@example.test"},"licenses":[],"backup_count":0,"storage_bytes":0,"cloud_enabled":true}`)
	}))
	defer server.Close()
	c, _ := New(server.URL)
	c.token = "session"
	a, err := c.Account(context.Background())
	if err != nil || a.User.ID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("account=%+v err=%v", a, err)
	}
}

func TestPublicKeysReadsRotatedKeyRingAndLegacyEndpoint(t *testing.T) {
	public1, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public2, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"public_key":%q,"public_keys":{"old-2026":%q,"new-2026":%q}}`, base64.RawURLEncoding.EncodeToString(public1), base64.RawURLEncoding.EncodeToString(public1), base64.RawURLEncoding.EncodeToString(public2))
	}))
	defer server.Close()
	c, _ := New(server.URL)
	keys, err := c.PublicKeys(context.Background())
	if err != nil || len(keys) != 2 || !keys["old-2026"].Equal(public1) || !keys["new-2026"].Equal(public2) {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
}

func TestLicenseActivationCarriesMatchingKeyRing(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(publicKey)
	keyID := "key-2026"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/licenses/activate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprintf(w, `{"id":"license-1","token":"payload.signature","installation_id":"install-1","status":"active","key_id":%q,"public_key":%q,"public_keys":{%q:%q}}`, keyID, encoded, keyID, encoded)
	}))
	defer server.Close()
	client, _ := New(server.URL)
	license, err := client.ActivateLicense(context.Background(), "purchase-code", "install-1")
	if err != nil || license.KeyID != keyID {
		t.Fatalf("license=%+v err=%v", license, err)
	}
	keys, err := DecodePublicKeys(license.PublicKeys)
	if err != nil || !keys[keyID].Equal(publicKey) {
		t.Fatalf("keys=%v err=%v", keys, err)
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
