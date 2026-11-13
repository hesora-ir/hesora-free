package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestPreferenceSettingsRoundTrip(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	preferences := []domain.Setting{
		{Key: "theme", Value: "dark"},
		{Key: "font_family", Value: "IRANSansX"},
		{Key: "font_size", Value: "12"},
		{Key: "app_icon_path", Value: "C:\\Icons\\invoice.ico"},
	}
	for _, preference := range preferences {
		if err := store.SetSetting(ctx, preference); err != nil {
			t.Fatalf("SetSetting(%q): %v", preference.Key, err)
		}
	}

	for _, want := range preferences {
		got, err := store.GetSetting(ctx, want.Key)
		if err != nil {
			t.Fatalf("GetSetting(%q): %v", want.Key, err)
		}
		if got.Value != want.Value {
			t.Errorf("setting %q = %q, want %q", want.Key, got.Value, want.Value)
		}
		if got.UpdatedAt.IsZero() {
			t.Errorf("setting %q has no updated timestamp", want.Key)
		}
	}
}
