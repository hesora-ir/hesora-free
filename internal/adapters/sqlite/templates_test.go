package sqlite

import (
	"context"
	"encoding/base64"
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
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6//8/IAAA//8GBgMAt2YRIQAAAABJRU5ErkJggg==")
	asset, err := domain.EncodeTemplateImage(png, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	tpl := domain.DocumentTemplate{Name: "قالب فروش", Prefix: "SALE-", ThankYou: "سپاس از خرید شما", LogoData: asset, SealData: asset, SignatureData: asset, ShowSeal: true, ShowSignature: true, PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 11, PrimaryColor: "#123456", AccentColor: "#456789", LetterheadID: headerID, BankAccountIDs: []int64{bankID}, ShowLogo: true, ShowTax: true, ShowBankAccounts: true}
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
	if got[0].ID != id || got[0].LetterheadID != headerID || got[0].Prefix != "SALE-" || got[0].ThankYou != "سپاس از خرید شما" || got[0].LogoData != asset || got[0].SealData != asset || got[0].SignatureData != asset || !got[0].ShowSeal || !got[0].ShowSignature || !reflect.DeepEqual(got[0].BankAccountIDs, []int64{bankID}) {
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
