// Package licensing verifies signed license claims and exposes feature entitlements.
// The application contains only a public key; private signing keys must remain offline.
package licensing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const ProductID = "hesora"
const maxTokenLength = 64 * 1024

// KeyID is the stable 16-hex-character identifier shared by the web signer
// and desktop verifier for an Ed25519 public key.
func KeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:8])
}

var (
	ErrMalformedLicense     = errors.New("ساختار لایسنس نامعتبر است")
	ErrInvalidSignature     = errors.New("امضای لایسنس معتبر نیست")
	ErrWrongProduct         = errors.New("این لایسنس برای این محصول صادر نشده است")
	ErrExpired              = errors.New("اعتبار لایسنس پایان یافته است")
	ErrNotYetValid          = errors.New("لایسنس هنوز معتبر نشده است")
	ErrInstallationMismatch = errors.New("لایسنس برای این نصب صادر نشده است")
	ErrInvalidClaims        = errors.New("اطلاعات لایسنس نامعتبر است")
)

type Claims struct {
	LicenseID      string   `json:"license_id"`
	KeyID          string   `json:"key_id,omitempty"`
	Product        string   `json:"product"`
	Subject        string   `json:"subject"`
	IssuedAt       int64    `json:"issued_at"`
	NotBefore      int64    `json:"not_before,omitempty"`
	ExpiresAt      int64    `json:"expires_at,omitempty"`
	InstallationID string   `json:"installation_id,omitempty"`
	Features       []string `json:"features"`
}

type Status string

const (
	StatusFree     Status = "free"
	StatusLicensed Status = "licensed"
	StatusExpired  Status = "expired"
)

type Result struct {
	Status   Status
	Claims   *Claims
	Features map[string]bool
	Reason   error
}

type Verifier struct {
	PublicKey      ed25519.PublicKey
	PublicKeys     map[string]ed25519.PublicKey
	Now            func() time.Time
	InstallationID string
	FreeFeatures   []string
}

// Verify expects base64url(payload).base64url(signature); signature is Ed25519
// over the exact decoded payload bytes. JSON is not re-serialized before verification.
func (v Verifier) Verify(token string) Result {
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	result := Result{Status: StatusFree, Features: makeFeatures(v.FreeFeatures)}
	token = strings.TrimSpace(token)
	if len(token) == 0 || len(token) > maxTokenLength {
		result.Reason = ErrMalformedLicense
		return result
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		result.Reason = ErrMalformedLicense
		return result
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		result.Reason = ErrMalformedLicense
		return result
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		result.Reason = ErrMalformedLicense
		return result
	}
	keys := []ed25519.PublicKey{}
	if claims.KeyID != "" {
		if len(claims.KeyID) > 128 {
			result.Reason = ErrInvalidClaims
			return result
		}
		if key := v.PublicKeys[claims.KeyID]; len(key) == ed25519.PublicKeySize {
			keys = append(keys, key)
		}
		// Older builds only carry one pinned key and can still activate before
		// downloading the key ring. Never use this fallback when a ring exists.
		if len(keys) == 0 && len(v.PublicKeys) == 0 && len(v.PublicKey) == ed25519.PublicKeySize {
			keys = append(keys, v.PublicKey)
		}
		if len(keys) == 0 && len(v.PublicKeys) == 1 {
			if key := v.PublicKeys["legacy"]; len(key) == ed25519.PublicKeySize {
				keys = append(keys, key)
			}
		}
	} else {
		if len(v.PublicKey) == ed25519.PublicKeySize {
			keys = append(keys, v.PublicKey)
		}
		ids := make([]string, 0, len(v.PublicKeys))
		for id := range v.PublicKeys {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if len(v.PublicKeys[id]) == ed25519.PublicKeySize {
				keys = append(keys, v.PublicKeys[id])
			}
		}
	}
	verified := false
	for _, key := range keys {
		if ed25519.Verify(key, payload, signature) {
			verified = true
			break
		}
	}
	if err != nil || !verified {
		result.Reason = ErrInvalidSignature
		return result
	}
	if strings.TrimSpace(claims.LicenseID) == "" || len(claims.LicenseID) > 200 ||
		strings.TrimSpace(claims.Subject) == "" || len(claims.Subject) > 300 ||
		claims.IssuedAt <= 0 || claims.Product == "" || len(claims.Features) > 100 ||
		len(claims.InstallationID) > 128 {
		result.Reason = ErrInvalidClaims
		return result
	}
	if claims.Product != ProductID {
		result.Reason = ErrWrongProduct
		return result
	}
	if claims.NotBefore > 0 && now.Unix() < claims.NotBefore {
		result.Reason = ErrNotYetValid
		return result
	}
	if claims.IssuedAt > now.Add(5*time.Minute).Unix() ||
		(claims.ExpiresAt > 0 && claims.ExpiresAt <= claims.IssuedAt) ||
		(claims.NotBefore > 0 && claims.ExpiresAt > 0 && claims.NotBefore >= claims.ExpiresAt) {
		result.Reason = ErrInvalidClaims
		return result
	}
	if claims.InstallationID != "" && claims.InstallationID != v.InstallationID {
		result.Reason = ErrInstallationMismatch
		return result
	}
	if claims.ExpiresAt > 0 && now.Unix() >= claims.ExpiresAt {
		result.Status = StatusExpired
		result.Claims = &claims
		result.Reason = ErrExpired
		return result
	}
	features := makeFeatures(v.FreeFeatures)
	for _, feature := range claims.Features {
		if strings.TrimSpace(feature) == "" || len(feature) > 100 {
			result.Reason = ErrInvalidClaims
			return result
		}
		features[feature] = true
	}
	result.Status = StatusLicensed
	result.Claims = &claims
	result.Features = features
	return result
}

func (r Result) HasFeature(feature string) bool { return r.Features[feature] }

func makeFeatures(features []string) map[string]bool {
	out := make(map[string]bool, len(features))
	for _, feature := range features {
		if feature = strings.TrimSpace(feature); feature != "" {
			out[feature] = true
		}
	}
	return out
}
