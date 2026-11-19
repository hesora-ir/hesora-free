package domain

import (
	"encoding/base64"
	"testing"
)

func validTemplate() DocumentTemplate {
	return DocumentTemplate{Name: "قالب اصلی", PageSize: "A4", Orientation: "portrait", FontFamily: "Tahoma", FontSize: 10, PrimaryColor: "#123456", AccentColor: "#ABCDEF", ShowLogo: true, ShowTax: true, ShowBankAccounts: true}
}

func TestDocumentTemplateValidation(t *testing.T) {
	if err := validTemplate().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := validTemplate()
	bad.PrimaryColor = "red"
	if bad.Validate() == nil {
		t.Fatal("expected invalid CSS color")
	}
	bad = validTemplate()
	bad.FontSize = 33
	if bad.Validate() == nil {
		t.Fatal("expected invalid font size")
	}
	bad = validTemplate()
	bad.BankAccountIDs = []int64{2, 2}
	if bad.Validate() == nil {
		t.Fatal("expected duplicate bank account error")
	}
}

func TestTemplateImageValidationAndEncoding(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6//8/IAAA//8GBgMAt2YRIQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	asset, err := EncodeTemplateImage(png, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	template := validTemplate()
	template.LogoData, template.SealData, template.SignatureData = asset, asset, asset
	if err = template.Validate(); err != nil {
		t.Fatal(err)
	}
	template.SignatureData = "data:image/svg+xml;base64,PHN2Zy8+"
	if template.Validate() == nil {
		t.Fatal("SVG template image must be rejected")
	}
	if _, err = EncodeTemplateImage(png, "image/gif"); err == nil {
		t.Fatal("unsupported image MIME type must be rejected")
	}
}

func TestLetterheadAndBankAccountValidation(t *testing.T) {
	if err := (Letterhead{Name: "سربرگ", LogoPath: "C:/logos/logo.png", Title: "شرکت"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (BankAccount{BankName: "بانک نمونه", AccountHolder: "شرکت نمونه", IBAN: "IR000000000000000000000000"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if (BankAccount{AccountHolder: "شرکت"}).Validate() == nil {
		t.Fatal("expected bank name error")
	}
}
