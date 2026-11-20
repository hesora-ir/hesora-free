// Package cloud connects the desktop application to the Hesora HTTP API.
// Session tokens live in memory only and are never included in database backups.
package cloud

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Client struct {
	baseURL, token string
	http           *http.Client
}
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("سرویس هسورا (%d): %s", e.Status, e.Message)
}
func IsForbidden(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == http.StatusForbidden
}

func IsConflict(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == http.StatusConflict
}

type License struct {
	ID               string            `json:"id"`
	Token            string            `json:"token"`
	InstallationID   string            `json:"installation_id"`
	ExpiresAt        int64             `json:"expires_at"`
	Status           string            `json:"status"`
	RemainingSeconds int64             `json:"remaining_seconds"`
	KeyID            string            `json:"key_id,omitempty"`
	PublicKey        string            `json:"public_key,omitempty"`
	PublicKeys       map[string]string `json:"public_keys,omitempty"`
	ServerTime       int64             `json:"server_time,omitempty"`
}

type TrialStatus struct {
	ExpiresAt  int64 `json:"expires_at"`
	ServerTime int64 `json:"server_time"`
}

func (c *Client) StartTrial(ctx context.Context, installationID, deviceFingerprint string) (License, error) {
	var license License
	err := c.json(ctx, http.MethodPost, "/licenses/trial", map[string]string{
		"installation_id":    installationID,
		"device_fingerprint": deviceFingerprint,
	}, &license)
	return license, err
}

func (c *Client) ValidateTrial(ctx context.Context, installationID, deviceFingerprint, token string) (TrialStatus, error) {
	var result TrialStatus
	err := c.json(ctx, http.MethodPost, "/licenses/trial/validate", map[string]string{
		"installation_id":    installationID,
		"device_fingerprint": deviceFingerprint,
		"token":              token,
	}, &result)
	return result, err
}

type Account struct {
	User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
	Licenses     []License `json:"licenses"`
	BackupCount  int       `json:"backup_count"`
	StorageBytes int64     `json:"storage_bytes"`
	CloudEnabled bool      `json:"cloud_enabled"`
}
type Backup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Checksum  string `json:"checksum"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"created_at"`
}

type WorkspacePaymentAllocation struct {
	DocumentID string `json:"document_id"`
	Number     string `json:"number,omitempty"`
	Amount     int64  `json:"amount"`
}

type WorkspacePayment struct {
	ID             string                       `json:"id"`
	ExternalRef    string                       `json:"external_ref"`
	Amount         int64                        `json:"amount"`
	Currency       string                       `json:"currency"`
	PaidAt         int64                        `json:"paid_at"`
	Method         string                       `json:"method"`
	Note           string                       `json:"note"`
	ReversedAt     *int64                       `json:"reversed_at,omitempty"`
	ReversalReason string                       `json:"reversal_reason,omitempty"`
	Allocations    []WorkspacePaymentAllocation `json:"allocations"`
}
type Document struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	Version      int64           `json:"version"`
	UpdatedAt    int64           `json:"updated_at"`
	Deleted      bool            `json:"deleted"`
	Data         json.RawMessage `json:"data"`
	ChangeCursor int64           `json:"change_cursor,omitempty"`
}
type documentChangesPage struct {
	Changes []Document `json:"changes"`
	Cursor  int64      `json:"cursor"`
	HasMore bool       `json:"has_more"`
}

func New(baseURL string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, err
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && local)) {
		return nil, errors.New("نشانی سرویس باید HTTPS باشد؛ HTTP فقط برای localhost مجاز است")
	}
	return &Client{baseURL: u.String(), http: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("تغییر مسیر سرویس مجاز نیست")
	}}}, nil
}
func (c *Client) request(ctx context.Context, method, path string, body io.Reader, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	// OpenLiteSpeed/PHP installations may reject chunked raw uploads. Backups
	// are regular files, so send their known size as Content-Length.
	if file, ok := body.(*os.File); ok {
		info, statErr := file.Stat()
		if statErr != nil {
			return statErr
		}
		req.ContentLength = info.Size()
	}
	req.Header.Set("X-Hesora-Client", "1")
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ارتباط با سرویس: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem struct {
			Error string `json:"error"`
		}
		json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&problem)
		if problem.Error == "" {
			problem.Error = response.Status
		}
		return &APIError{response.StatusCode, problem.Error}
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(response.Body, 25<<20)).Decode(out)
	}
	return nil
}
func (c *Client) json(ctx context.Context, method, path string, in, out any) error {
	var data []byte
	var err error
	if in != nil {
		data, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	return c.request(ctx, method, path, bytes.NewReader(data), nil, out)
}
func (c *Client) Login(ctx context.Context, email, password string) error {
	var result struct {
		Token string `json:"token"`
	}
	if err := c.json(ctx, "POST", "/auth/login", map[string]string{"email": email, "password": password}, &result); err != nil {
		return err
	}
	if result.Token == "" {
		return errors.New("پاسخ ورود نامعتبر است")
	}
	c.token = result.Token
	return nil
}
func (c *Client) DeviceLogin(ctx context.Context, licenseToken, installation string) error {
	var result struct {
		Token string `json:"token"`
	}
	if err := c.json(ctx, "POST", "/auth/device-login", map[string]string{"token": licenseToken, "installation_id": installation}, &result); err != nil {
		return err
	}
	if result.Token == "" {
		return errors.New("پاسخ ورود دستگاه نامعتبر است")
	}
	c.token = result.Token
	return nil
}
func (c *Client) Logout(ctx context.Context) error {
	err := c.json(ctx, "POST", "/auth/logout", nil, nil)
	c.token = ""
	return err
}
func (c *Client) Account(ctx context.Context) (Account, error) {
	var a Account
	err := c.json(ctx, "GET", "/me", nil, &a)
	return a, err
}

func (c *Client) MagicLink(ctx context.Context) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	if err := c.json(ctx, http.MethodPost, "/account/magic-link", map[string]any{}, &result); err != nil {
		return "", err
	}
	if !strings.HasPrefix(result.URL, c.baseURL+"/magic/") {
		return "", errors.New("Ù¾Ø§Ø³Ø® Ù¾ÛŒÙˆÙ†Ø¯ ÙˆØ±ÙˆØ¯ Ù†Ø§Ù…Ø¹ØªØ¨Ø± Ø§Ø³Øª")
	}
	return result.URL, nil
}

func (c *Client) CreateSupportTicket(ctx context.Context, subject, body string) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	err := c.json(ctx, http.MethodPost, "/support/tickets", map[string]string{"subject": subject, "body": body}, &result)
	return result.ID, err
}
func (c *Client) PublicKey(ctx context.Context) (ed25519.PublicKey, error) {
	keys, err := c.PublicKeys(ctx)
	if err != nil {
		return nil, err
	}
	if key, ok := keys["legacy"]; ok {
		return key, nil
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, errors.New("کلید عمومی دریافت‌شده از سرور معتبر نیست")
	}
	return keys[ids[0]], nil
}

func (c *Client) PublicKeys(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	var response struct {
		PublicKey  string            `json:"public_key"`
		PublicKeys map[string]string `json:"public_keys"`
	}
	if err := c.json(ctx, "GET", "/licenses/public-key", nil, &response); err != nil {
		return nil, err
	}
	if len(response.PublicKeys) == 0 && response.PublicKey != "" {
		response.PublicKeys = map[string]string{"legacy": response.PublicKey}
	}
	return DecodePublicKeys(response.PublicKeys)
}

func DecodePublicKeys(encoded map[string]string) (map[string]ed25519.PublicKey, error) {
	decode := func(value string) ([]byte, error) {
		key, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			key, err = base64.StdEncoding.DecodeString(value)
		}
		return key, err
	}
	if len(encoded) == 0 || len(encoded) > 32 {
		return nil, errors.New("حلقهٔ کلید عمومی دریافت‌شده از سرور معتبر نیست")
	}
	keys := make(map[string]ed25519.PublicKey, len(encoded))
	for id, value := range encoded {
		key, err := decode(value)
		if id == "" || len(id) > 128 || err != nil || len(key) != ed25519.PublicKeySize {
			return nil, errors.New("حلقهٔ کلید عمومی دریافت‌شده از سرور معتبر نیست")
		}
		keys[id] = ed25519.PublicKey(key)
	}
	return keys, nil
}
func (c *Client) ActivateLicense(ctx context.Context, code, installation string) (License, error) {
	var license License
	err := c.json(ctx, "POST", "/licenses/activate", map[string]string{"code": code, "installation_id": installation}, &license)
	return license, err
}
func (c *Client) ValidateLicense(ctx context.Context, token, installation string) (License, error) {
	var l License
	err := c.json(ctx, "POST", "/licenses/validate", map[string]string{"token": token, "installation_id": installation}, &l)
	return l, err
}
func (c *Client) Backups(ctx context.Context) ([]Backup, error) {
	var b []Backup
	err := c.json(ctx, "GET", "/backups", nil, &b)
	return b, err
}

// DownloadBackup writes a user-owned cloud backup only after verifying its
// SHA-256 checksum. The temporary file is removed if the transfer is invalid.
func (c *Client) DownloadBackup(ctx context.Context, id, path string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(path) == "" {
		return errors.New("شناسه بکاپ و مسیر ذخیره‌سازی الزامی است")
	}
	response, err := c.downloadRequest(ctx, "/backups/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".hesora-backup-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(response.Body, (50<<20)+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > 50<<20 {
		return errors.New("حجم بکاپ از سقف ۵۰ مگابایت بیشتر است")
	}
	expected := strings.TrimSpace(response.Header.Get("X-Backup-SHA256"))
	if len(expected) != sha256.Size*2 || !strings.EqualFold(expected, hex.EncodeToString(hash.Sum(nil))) {
		return errors.New("بررسی سلامت بکاپ ناموفق بود؛ فایل ذخیره نشد")
	}
	return os.Rename(tmpPath, path)
}

func (c *Client) DeleteBackup(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("شناسه بکاپ الزامی است")
	}
	return c.json(ctx, "DELETE", "/backups/"+url.PathEscape(id), nil, nil)
}

func (c *Client) downloadRequest(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Hesora-Client", "1")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("دریافت بکاپ از سرویس: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		var problem struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&problem)
		if problem.Error == "" {
			problem.Error = response.Status
		}
		return nil, &APIError{response.StatusCode, problem.Error}
	}
	return response, nil
}

func (c *Client) UploadBackup(ctx context.Context, path string) (Backup, error) {
	var b Backup
	file, err := os.Open(path)
	if err != nil {
		return b, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return b, err
	}
	if stat.Size() > 50<<20 {
		return b, errors.New("حداکثر اندازه بکاپ ۵۰ مگابایت است")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return b, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return b, err
	}
	err = c.request(ctx, "POST", "/backups", file, map[string]string{"Content-Type": "application/octet-stream", "X-Backup-Name": filepath.Base(path), "X-Backup-SHA256": hex.EncodeToString(hash.Sum(nil))}, &b)
	return b, err
}

func (c *Client) ImportBackupToWorkspace(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("شناسه بکاپ الزامی است")
	}
	return c.json(ctx, http.MethodPost, "/backups/"+url.PathEscape(id)+"/import", map[string]any{}, nil)
}

func (c *Client) ShareInvoice(ctx context.Context, data map[string]any) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := c.json(ctx, http.MethodPost, "/workspace/invoices/share", data, &result)
	if err == nil {
		parsed, parseErr := url.Parse(result.URL)
		base, _ := url.Parse(c.baseURL)
		if parseErr != nil || parsed.Scheme != base.Scheme || parsed.Host != base.Host || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/share/") {
			err = errors.New("نشانی پیوند اشتراک‌گذاری معتبر نیست")
		}
	}
	return result.URL, err
}

// SaveWorkspacePayment uses the server's stable payment ID for idempotent sync.
func (c *Client) SaveWorkspacePayment(ctx context.Context, payment WorkspacePayment) (bool, error) {
	var result struct {
		Idempotent bool `json:"idempotent"`
	}
	err := c.json(ctx, http.MethodPost, "/workspace/payments", payment, &result)
	return result.Idempotent, err
}

func (c *Client) WorkspacePayments(ctx context.Context) ([]WorkspacePayment, error) {
	var result struct {
		Payments []WorkspacePayment `json:"payments"`
	}
	err := c.json(ctx, http.MethodGet, "/workspace/payments", nil, &result)
	return result.Payments, err
}

func (c *Client) ReverseWorkspacePayment(ctx context.Context, id, reason string) error {
	if len(id) != 32 || strings.TrimSpace(reason) == "" {
		return errors.New("invalid payment ID or reversal reason")
	}
	return c.json(ctx, http.MethodPost, "/workspace/payments/"+url.PathEscape(id)+"/reverse", map[string]string{"reason": reason}, nil)
}

func (c *Client) Documents(ctx context.Context) ([]Document, error) {
	var docs []Document
	err := c.json(ctx, "GET", "/cloud/documents", nil, &docs)
	return docs, err
}

// DocumentsSince retrieves only documents changed after the durable server cursor.
// The server coalesces repeated changes to the same document and retains tombstones.
func (c *Client) DocumentsSince(ctx context.Context, after int64) ([]Document, int64, error) {
	if after < 0 {
		return nil, after, errors.New("invalid document change cursor")
	}
	all := make([]Document, 0)
	for {
		var page documentChangesPage
		path := fmt.Sprintf("/cloud/document-changes?after=%d&limit=200", after)
		if err := c.json(ctx, http.MethodGet, path, nil, &page); err != nil {
			return nil, after, err
		}
		if page.Cursor < after || (page.HasMore && page.Cursor == after) {
			return nil, after, errors.New("invalid document change cursor response")
		}
		all = append(all, page.Changes...)
		after = page.Cursor
		if !page.HasMore {
			return all, after, nil
		}
	}
}

// PutDocument uses optimistic concurrency. A stale Version returns HTTP 409;
// callers must fetch the remote version and resolve the conflict before retrying.
func (c *Client) PutDocument(ctx context.Context, d Document) (Document, error) {
	var out Document
	err := c.json(ctx, "PUT", "/cloud/documents/"+url.PathEscape(d.ID), map[string]any{"kind": d.Kind, "version": d.Version, "deleted": d.Deleted, "data": d.Data}, &out)
	return out, err
}

// PutDocumentDesktopWins retries one optimistic conflict against the latest
// remote version. The UI asks the user to confirm this policy before sync.
func (c *Client) PutDocumentDesktopWins(ctx context.Context, d Document) (Document, error) {
	out, _, err := c.PutDocumentThreeWay(ctx, d, nil, true)
	return out, err
}

// PutDocumentThreeWay merges non-overlapping fields against the last common
// snapshot, then retries once at the latest remote version. preferLocal must
// only be true after the user has accepted the desktop-wins conflict policy.
func (c *Client) PutDocumentThreeWay(ctx context.Context, d Document, base json.RawMessage, preferLocal bool) (Document, []string, error) {
	out, err := c.PutDocument(ctx, d)
	if !IsConflict(err) {
		return out, nil, err
	}
	latest, err := c.Document(ctx, d.ID)
	if err != nil {
		return Document{}, nil, err
	}
	conflicts := []string{}
	if d.Deleted != latest.Deleted {
		conflicts = []string{"$ (delete/update conflict)"}
	} else if !d.Deleted && !latest.Deleted {
		if len(bytes.TrimSpace(base)) == 0 {
			// Existing installs may not yet have a common-snapshot column value.
			// Preserve the established user-approved desktop-wins behavior until
			// this document has completed one sync and stores a new base snapshot.
			conflicts = []string{"$ (missing common snapshot)"}
		} else {
			merged, paths, mergeErr := MergeDocumentData(base, d.Data, latest.Data, preferLocal)
			if mergeErr != nil {
				return Document{}, nil, mergeErr
			}
			d.Data, conflicts = merged, paths
		}
	}
	d.Version = latest.Version
	out, err = c.PutDocument(ctx, d)
	if err != nil {
		return Document{}, conflicts, err
	}
	return out, conflicts, nil
}

func (c *Client) Document(ctx context.Context, id string) (Document, error) {
	if id == "" || strings.ContainsAny(id, "/?#") {
		return Document{}, errors.New("invalid document ID")
	}
	var doc Document
	err := c.json(ctx, http.MethodGet, "/cloud/documents/"+url.PathEscape(id), nil, &doc)
	return doc, err
}
