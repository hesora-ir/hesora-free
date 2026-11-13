package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestIssuerProfilePersistsRepeatableRows(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "issuer-profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	profile := domain.IssuerProfile{
		Type:      domain.IssuerTypeIndividual,
		Name:      "نام کامل",
		FirstName: "نام",
		LastName:  "خانوادگی",
		Phones:    []string{"phone-a", "phone-b"},
		BankDetails: []domain.IssuerBankDetails{
			{CardNumber: "card-test", AccountNumber: "account-test", IBAN: "iban-test"},
			{AccountNumber: "second-account"},
		},
	}
	id, err := store.SaveIssuerProfile(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.IssuerProfileByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != domain.IssuerTypeIndividual || got.FirstName != "نام" || got.LastName != "خانوادگی" {
		t.Fatalf("unexpected profile: %+v", got)
	}
	if len(got.Phones) != 2 || got.Phones[1] != "phone-b" {
		t.Fatalf("unexpected phones: %+v", got.Phones)
	}
	if len(got.BankDetails) != 2 || got.BankDetails[0].IBAN != "iban-test" || got.BankDetails[1].AccountNumber != "second-account" {
		t.Fatalf("unexpected bank rows: %+v", got.BankDetails)
	}
	got.Phones = []string{"phone-c"}
	got.BankDetails = []domain.IssuerBankDetails{{CardNumber: "card-updated"}}
	if _, err := store.SaveIssuerProfile(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, err = store.IssuerProfileByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Phones) != 1 || got.Phones[0] != "phone-c" || len(got.BankDetails) != 1 || got.BankDetails[0].CardNumber != "card-updated" {
		t.Fatalf("unexpected updated child rows: %+v", got)
	}
}
