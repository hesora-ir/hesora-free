package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestIssuerProfileRoundTrip(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	id, err := store.SaveIssuerProfile(ctx, domain.IssuerProfile{Name: "فروشگاه آزمایشی", LegalName: "شرکت نمونه", Phone: "021"})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := store.IssuerProfileByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "فروشگاه آزمایشی" || profile.LegalName != "شرکت نمونه" || profile.Phone != "021" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	profile.Phone = "026"
	if _, err := store.SaveIssuerProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	profile, err = store.IssuerProfileByID(ctx, id)
	if err != nil || profile.Phone != "026" {
		t.Fatalf("updated profile = %+v, err = %v", profile, err)
	}
}

func TestSettingsUpsert(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	for _, value := range []string{"IRT", "IRR"} {
		if err := store.SetSetting(ctx, domain.Setting{Key: "default_currency", Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.GetSetting(ctx, "default_currency")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "IRR" || got.UpdatedAt.IsZero() {
		t.Fatalf("unexpected setting: %+v", got)
	}
}
