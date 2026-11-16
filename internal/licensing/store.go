package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	data, err := os.ReadFile(filepath.Join(s.Directory, publicKeyFileName))
	if err != nil {
		return nil, err
	}
	if len(data) != ed25519.PublicKeySize {
		return nil, errors.New("کلید عمومی ذخیره‌شدهٔ لایسنس معتبر نیست")
	}
	return ed25519.PublicKey(data), nil
}

func (s FileStore) SavePublicKey(key ed25519.PublicKey) error {
	if len(key) != ed25519.PublicKeySize {
		return errors.New("کلید عمومی لایسنس معتبر نیست")
	}
	return atomicWrite(s.Directory, publicKeyFileName, key)
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
