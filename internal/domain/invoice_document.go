package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type NumberingConfig struct {
	Series  string
	Prefix  string
	Suffix  string
	StartAt int64
	Width   int
}

func (c NumberingConfig) Validate() error {
	if strings.TrimSpace(c.Series) == "" || len(c.Series) > 100 {
		return errors.New("سری شماره‌گذاری نامعتبر است")
	}
	if c.StartAt < 1 {
		return errors.New("شماره شروع باید مثبت باشد")
	}
	if c.Width < 1 || c.Width > 12 {
		return errors.New("طول شماره باید بین ۱ تا ۱۲ رقم باشد")
	}
	if len(c.Prefix) > 100 || len(c.Suffix) > 100 {
		return errors.New("پیشوند یا پسوند بیش از حد مجاز است")
	}
	return nil
}

type InvoiceDocument struct {
	ID                   int64
	Number               string
	Kind                 string
	DocumentType         string
	Status               InvoiceStatus
	Currency             Currency
	CustomerID           int64
	IssuerProfileID      int64
	CustomerSnapshot     string
	IssuerSnapshot       string
	IssueDate            time.Time
	DueDate              time.Time
	Note                 string
	BankAccountsSnapshot string
	Items                []InvoiceItem
	AdditionalCharges    []AdditionalCharge
	InvoiceDiscount      Discount
	TaxRateBPS           int64
	Numbering            NumberingConfig
}

type AdditionalCharge struct {
	Description string
	Amount      Money
}

func DocumentTypeLabelFA(doc InvoiceDocument) string {
	return DocumentTypeNameFA(doc.DocumentType, doc.Kind)
}

func DocumentTypeNameFA(documentType, kind string) string {
	switch documentType {
	case "credit":
		return "فاکتور اعتباری"
	case "service":
		return "فاکتور خدمات"
	case "quotation":
		return "اعلام قیمت"
	case "corrective":
		return "فاکتور اصلاحی"
	case "proforma":
		return "پیش‌فاکتور"
	case "sales":
		return "فاکتور فروش"
	default:
		if kind == "proforma" {
			return "پیش‌فاکتور"
		}
		return "فاکتور فروش"
	}
}

func (d InvoiceDocument) Validate() error {
	if d.Kind != "proforma" && d.Kind != "invoice" && d.Kind != "settlement" {
		return errors.New("نوع سند نامعتبر است")
	}
	if d.Currency != CurrencyRial && d.Currency != CurrencyToman {
		return errors.New("واحد پول نامعتبر است")
	}
	if d.CustomerID < 0 || d.IssuerProfileID < 0 {
		return errors.New("شناسه مشتری یا صادرکننده نامعتبر است")
	}
	if d.IssueDate.IsZero() {
		return errors.New("تاریخ صدور الزامی است")
	}
	if !d.DueDate.IsZero() && d.DueDate.Before(d.IssueDate) {
		return errors.New("سررسید نمی‌تواند پیش از تاریخ صدور باشد")
	}
	if len(d.Note) > 10000 {
		return errors.New("یادداشت بیش از حد مجاز است")
	}
	if len(d.BankAccountsSnapshot) > 20000 {
		return errors.New("اطلاعات حساب‌های بانکی بیش از حد مجاز است")
	}
	if d.DocumentType != "" && d.DocumentType != "sales" && d.DocumentType != "credit" && d.DocumentType != "service" && d.DocumentType != "quotation" && d.DocumentType != "corrective" && d.DocumentType != "proforma" {
		return errors.New("نوع فاکتور نامعتبر است")
	}
	if len(d.AdditionalCharges) > 100 {
		return errors.New("حداکثر ۱۰۰ هزینهٔ اضافه قابل ثبت است")
	}
	for i, c := range d.AdditionalCharges {
		if strings.TrimSpace(c.Description) == "" || len(c.Description) > 200 || c.Amount < 0 {
			return errors.New("هزینهٔ اضافهٔ ردیف " + strconv.Itoa(i+1) + " نامعتبر است")
		}
	}
	if d.CustomerSnapshot == "" {
		return errors.New("تصویر اطلاعات مشتری الزامی است")
	}
	if d.IssuerSnapshot == "" {
		return errors.New("تصویر اطلاعات صادرکننده الزامی است")
	}
	if err := d.Numbering.Validate(); err != nil {
		return err
	}
	_, err := CalculateFinancialsWithCharges(d.Items, d.InvoiceDiscount, d.TaxRateBPS, d.AdditionalCharges)
	return err
}

type InvoiceListItem struct {
	ID           int64
	Number       string
	Kind         string
	DocumentType string
	Status       string
	Currency     string
	Customer     string
	IssueDate    string
	Total        int64
	Balance      int64
}
