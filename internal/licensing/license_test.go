package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, Verifier) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	v := Verifier{PublicKey: pub, InstallationID: "install-1", FreeFeatures: []string{"basic.invoice"}, Now: func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) }}
	return pub, priv, v
}

func validClaims() Claims {
	return Claims{LicenseID: "LIC-1", Product: ProductID, Subject: "Customer", IssuedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix(), ExpiresAt: time.Date(2027, 10, 1, 0, 0, 0, 0, time.UTC).Unix(), InstallationID: "install-1", Features: []string{"export.pdf", "advanced.templates"}}
}

// signClaimsForTest is deliberately test-only; production signing keys stay outside this package.
func signClaimsForTest(t *testing.T, claims Claims, privateKey ed25519.PrivateKey) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestVerifySignedLicenseAndFeatures(t *testing.T) {
	_, priv, v := setup(t)
	got := v.Verify(signClaimsForTest(t, validClaims(), priv))
	if got.Status != StatusLicensed || got.Reason != nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !got.HasFeature("basic.invoice") || !got.HasFeature("export.pdf") || !got.HasFeature("advanced.templates") {
		t.Fatalf("missing feature grants: %+v", got.Features)
	}
}

func TestVerifierAcceptsRotatedKeyByIDAndLegacyKey(t *testing.T) {
	oldPub, _, v := setup(t)
	newPub, newPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	v.PublicKeys = map[string]ed25519.PublicKey{"old-2026": oldPub, "new-2026": newPub}
	c := validClaims()
	c.KeyID = "new-2026"
	if got := v.Verify(signClaimsForTest(t, c, newPriv)); got.Status != StatusLicensed {
		t.Fatalf("new key rejected: %+v", got)
	}
	_, oldPriv, oldVerifier := setup(t)
	oldVerifier.PublicKeys = v.PublicKeys
	if got := oldVerifier.Verify(signClaimsForTest(t, validClaims(), oldPriv)); got.Status != StatusLicensed {
		t.Fatalf("legacy token rejected after rotation: %+v", got)
	}
	c.KeyID = "retired-unknown"
	if got := v.Verify(signClaimsForTest(t, c, newPriv)); !errors.Is(got.Reason, ErrInvalidSignature) {
		t.Fatalf("unknown key ID accepted: %+v", got)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	_, priv, v := setup(t)
	parts := strings.Split(signClaimsForTest(t, validClaims(), priv), ".")
	parts[0] = "A" + parts[0][1:]
	got := v.Verify(strings.Join(parts, "."))
	if got.Reason == nil {
		t.Fatal("expected invalid signature")
	}
}

func TestVerifyExpiredLicenseKeepsFreeFeatures(t *testing.T) {
	_, priv, v := setup(t)
	c := validClaims()
	c.ExpiresAt = v.Now().Unix()
	got := v.Verify(signClaimsForTest(t, c, priv))
	if got.Status != StatusExpired || !errors.Is(got.Reason, ErrExpired) {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !got.HasFeature("basic.invoice") || got.HasFeature("export.pdf") {
		t.Fatalf("expired license should not grant paid features: %+v", got.Features)
	}
}

func TestVerifyRejectsWrongProductAndInstallation(t *testing.T) {
	_, priv, v := setup(t)
	c := validClaims()
	c.Product = "other"
	if got := v.Verify(signClaimsForTest(t, c, priv)); !errors.Is(got.Reason, ErrWrongProduct) {
		t.Fatalf("got %v", got.Reason)
	}
	c = validClaims()
	c.InstallationID = "different-install"
	if got := v.Verify(signClaimsForTest(t, c, priv)); !errors.Is(got.Reason, ErrInstallationMismatch) {
		t.Fatalf("got %v", got.Reason)
	}
}

func TestVerifyNotYetValidAndMalformed(t *testing.T) {
	_, priv, v := setup(t)
	c := validClaims()
	c.NotBefore = v.Now().Add(time.Hour).Unix()
	if got := v.Verify(signClaimsForTest(t, c, priv)); !errors.Is(got.Reason, ErrNotYetValid) {
		t.Fatalf("got %v", got.Reason)
	}
	if got := v.Verify("not-a-token"); !errors.Is(got.Reason, ErrMalformedLicense) {
		t.Fatalf("got %v", got.Reason)
	}
}
