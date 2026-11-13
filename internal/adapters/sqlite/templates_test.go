package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestTemplateAndBrandingPersistence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "templates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	headerID, err := store.SaveLetterhead(ctx, domain.Letterhead{Name: "سربرگ فروش", LogoPath: "C:/brand/logo.png", Title: "شرکت نمونه", Subtitle: "فروش و خدمات"})
	if err != nil {
		t.Fatal(err)
	}
	bankID, err := store.SaveBankAccount(ctx, domain.BankAccount{BankName: "بانک نمونه", AccountHolder: "شرکت نمونه", AccountNumber: "12345", CardNumber: "1111222233334444", IBAN: "IR000000000000000000000000"})
	if err != nil {
		t.Fatal(err)
	}
	tpl := domain.DocumentTemplate{Name: "قالب فروش", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 11, PrimaryColor: "#123456", AccentColor: "#456789", LetterheadID: headerID, BankAccountIDs: []int64{bankID}, ShowLogo: true, ShowTax: true, ShowBankAccounts: true}
	id, err := store.SaveDocumentTemplate(ctx, tpl)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ListDocumentTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("templates count=%d", len(got))
	}
	if got[0].ID != id || got[0].LetterheadID != headerID || !reflect.DeepEqual(got[0].BankAccountIDs, []int64{bankID}) {
		t.Fatalf("unexpected template: %+v", got[0])
	}
	headers, err := store.ListLetterheads(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 || headers[0].Title != "شرکت نمونه" {
		t.Fatalf("unexpected headers: %+v", headers)
	}
	banks, err := store.ListBankAccounts(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(banks) != 1 || banks[0].IBAN != "IR000000000000000000000000" {
		t.Fatalf("unexpected accounts: %+v", banks)
	}
}

func TestSaveTemplateRollsBackInvalidBankAccountReference(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "templates-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tpl := domain.DocumentTemplate{Name: "قالب نامعتبر", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#123456", AccentColor: "#456789", BankAccountIDs: []int64{99999}}
	if _, err := store.SaveDocumentTemplate(ctx, tpl); err == nil {
		t.Fatal("expected invalid account reference to fail")
	}
	got, err := store.ListDocumentTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("template transaction should roll back, got %d rows", len(got))
	}
}
