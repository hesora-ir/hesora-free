package sqlite

import (
	"context"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestIssuerIBANRewriteReplacesStaleBankLabel(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	profile := domain.IssuerProfile{Type: domain.IssuerTypeIndividual, Name: "فروشنده", FirstName: "فروشنده",
		BankDetails: []domain.IssuerBankDetails{{BankName: "بانک قبلی", IBAN: "IR۸۲ ۰۵۴۰ ۱۰۲۶ ۸۰۰۲ ۰۸۱۷ ۹۰۹۰ ۰۲"}}}
	id, err := s.SaveIssuerProfile(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.IssuerProfileByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BankDetails) != 1 || got.BankDetails[0].IBAN != "IR820540102680020817909002" || got.BankDetails[0].BankName != "بانک پارسیان" {
		t.Fatalf("profile=%+v", got)
	}
	got.BankDetails[0].BankName = "نام مانده از قبل"
	got.BankDetails[0].IBAN = "۸۲۰۵۴۰۱۰۲۶۸۰۰۲۰۸۱۷۹۰۹۰۰۲"
	if _, err := s.SaveIssuerProfile(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, err = s.IssuerProfileByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.BankDetails[0].BankName != "بانک پارسیان" || got.BankDetails[0].IBAN != "IR820540102680020817909002" {
		t.Fatalf("updated profile=%+v", got)
	}
}
