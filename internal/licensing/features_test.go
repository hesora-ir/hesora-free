package licensing

import (
	"errors"
	"testing"
)

func TestDefaultFreeFeatures(t *testing.T) {
	features := DefaultFreeFeatures()
	if len(features) == 0 {
		t.Fatal("expected free features")
	}
	for _, id := range features {
		if id != "basic.invoice" && id != "basic.customers" {
			t.Fatalf("unexpected free feature %q", id)
		}
	}
}

func TestRequireFeature(t *testing.T) {
	result := Result{Status: StatusFree, Features: map[string]bool{"basic.invoice": true}}
	if err := RequireFeature(result, "basic.invoice"); err != nil {
		t.Fatal(err)
	}
	if err := RequireFeature(result, "export.pdf"); !errors.Is(err, ErrFeatureNotLicensed) {
		t.Fatalf("got %v", err)
	}
}
