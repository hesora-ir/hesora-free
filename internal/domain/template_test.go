package domain

import "testing"

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
