package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestLocalCustomerAndIssuerCountsAreNotFreeTierCapped(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "unlimited.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 51; i++ {
		if _, err := store.CreateCustomer(ctx, domain.Customer{Name: fmt.Sprintf("Customer %02d", i)}); err != nil {
			t.Fatalf("customer %d: %v", i+1, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := store.SaveIssuerProfile(ctx, domain.IssuerProfile{Name: fmt.Sprintf("Store %02d", i)}); err != nil {
			t.Fatalf("issuer %d: %v", i+1, err)
		}
	}
}
