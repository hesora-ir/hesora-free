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

type License struct {
	ID               string `json:"id"`
	Token            string `json:"token"`
	InstallationID   string `json:"installation_id"`
	ExpiresAt        int64  `json:"expires_at"`
	Status           string `json:"status"`
	RemainingSeconds int64  `json:"remaining_seconds"`
}
type Account struct {
	User struct {
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
type Document struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Version   int64           `json:"version"`
	UpdatedAt int64           `json:"updated_at"`
	Deleted   bool            `json:"deleted"`
	Data      json.RawMessage `json:"data"`
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
func (c *Client) PublicKey(ctx context.Context) (ed25519.PublicKey, error) {
	var response struct {
		PublicKey string `json:"public_key"`
	}
	if err := c.json(ctx, "GET", "/licenses/public-key", nil, &response); err != nil {
		return nil, err
	}
	key, err := base64.RawURLEncoding.DecodeString(response.PublicKey)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(response.PublicKey)
	}
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("کلید عمومی دریافت‌شده از سرور معتبر نیست")
	}
	return ed25519.PublicKey(key), nil
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
func (c *Client) Documents(ctx context.Context) ([]Document, error) {
	var docs []Document
	err := c.json(ctx, "GET", "/cloud/documents", nil, &docs)
	return docs, err
}

// PutDocument uses optimistic concurrency. A stale Version returns HTTP 409;
// callers must fetch the remote version and resolve the conflict before retrying.
func (c *Client) PutDocument(ctx context.Context, d Document) (Document, error) {
	var out Document
	err := c.json(ctx, "PUT", "/cloud/documents/"+url.PathEscape(d.ID), map[string]any{"kind": d.Kind, "version": d.Version, "deleted": d.Deleted, "data": d.Data}, &out)
	return out, err
}
