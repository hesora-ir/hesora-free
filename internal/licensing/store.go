package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	licenseFileName      = "license.token"
	installationFileName = "installation.id"
	publicKeyFileName    = "license-public.key"
)

type FileStore struct{ Directory string }

func (s FileStore) ReadToken() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.Directory, licenseFileName))
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", ErrMalformedLicense
	}
	return token, nil
}

func (s FileStore) SaveToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return ErrMalformedLicense
	}
	return atomicWrite(s.Directory, licenseFileName, []byte(token+"\n"))
}

func (s FileStore) ClearToken() error {
	err := os.Remove(filepath.Join(s.Directory, licenseFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s FileStore) ReadPublicKey() (ed25519.PublicKey, error) {
	keys, err := s.ReadPublicKeys()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("کلید عمومی ذخیره‌شدهٔ لایسنس معتبر نیست")
	}
	if key, ok := keys["legacy"]; ok {
		return key, nil
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return keys[ids[0]], nil
	}
	return nil, errors.New("کلید عمومی ذخیره‌شدهٔ لایسنس معتبر نیست")
}

// ReadPublicKeys accepts the legacy raw single-key file and the versioned map
// written by SavePublicKeys. The public key ring is not secret.
func (s FileStore) ReadPublicKeys() (map[string]ed25519.PublicKey, error) {
	data, err := os.ReadFile(filepath.Join(s.Directory, publicKeyFileName))
	if err != nil {
		return nil, err
	}
	if len(data) == ed25519.PublicKeySize {
		return map[string]ed25519.PublicKey{"legacy": ed25519.PublicKey(data)}, nil
	}
	var encoded map[string]string
	if err := json.Unmarshal(data, &encoded); err != nil || len(encoded) == 0 || len(encoded) > 32 {
		return nil, errors.New("حلقهٔ کلید عمومی ذخیره‌شدهٔ لایسنس معتبر نیست")
	}
	keys := make(map[string]ed25519.PublicKey, len(encoded))
	for id, value := range encoded {
		key, decodeErr := base64.RawURLEncoding.DecodeString(value)
		if id == "" || len(id) > 128 || decodeErr != nil || len(key) != ed25519.PublicKeySize {
			return nil, errors.New("حلقهٔ کلید عمومی ذخیره‌شدهٔ لایسنس معتبر نیست")
		}
		keys[id] = ed25519.PublicKey(key)
	}
	return keys, nil
}

func (s FileStore) SavePublicKey(key ed25519.PublicKey) error {
	if len(key) != ed25519.PublicKeySize {
		return errors.New("کلید عمومی لایسنس معتبر نیست")
	}
	return atomicWrite(s.Directory, publicKeyFileName, key)
}

func (s FileStore) SavePublicKeys(keys map[string]ed25519.PublicKey) error {
	if len(keys) == 0 || len(keys) > 32 {
		return errors.New("حلقهٔ کلید عمومی لایسنس معتبر نیست")
	}
	encoded := make(map[string]string, len(keys))
	for id, key := range keys {
		if id == "" || len(id) > 128 || len(key) != ed25519.PublicKeySize {
			return errors.New("حلقهٔ کلید عمومی لایسنس معتبر نیست")
		}
		encoded[id] = base64.RawURLEncoding.EncodeToString(key)
	}
	data, err := json.Marshal(encoded)
	if err != nil {
		return err
	}
	return atomicWrite(s.Directory, publicKeyFileName, data)
}

// InstallationID returns a stable random local identifier. It is a privacy-friendlier
// binding than collecting hardware serials, but is not a tamper-proof machine identity.
func (s FileStore) InstallationID() (string, error) {
	path := filepath.Join(s.Directory, installationFileName)
	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if len(id) == 32 {
			if _, decodeErr := hex.DecodeString(id); decodeErr == nil {
				return id, nil
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("ساخت شناسه نصب ناموفق بود: %w", err)
	}
	id := hex.EncodeToString(random)
	if err := atomicWrite(s.Directory, installationFileName, []byte(id+"\n")); err != nil {
		return "", err
	}
	return id, nil
}

func atomicWrite(directory, name string, data []byte) error {
	if strings.TrimSpace(directory) == "" {
		return errors.New("مسیر ذخیره‌سازی لایسنس الزامی است")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".license-*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(directory, name)); err != nil {
		return err
	}
	return nil
}
