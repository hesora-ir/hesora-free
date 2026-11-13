package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	CustomerTypeIndividual = "individual"
	CustomerTypeCompany    = "company"
)

type Customer struct {
	ID                  int64
	Type                string
	Name                string
	FirstName           string
	LastName            string
	LegalName           string
	TaxID               string
	Phone               string // Legacy primary phone.
	Phones              []string
	Email               string
	Address             string // Legacy primary address.
	Addresses           []string
	InvoiceCount        int
	IssuedInvoiceCount  int
	OpenInvoiceCount    int
	SettledInvoiceCount int
	ProformaCount       int
	IssuedTotalIRR      int64
	IssuedTotalIRT      int64
	DebtIRR             int64
	DebtIRT             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CustomerInvoice struct {
	ID        int64
	Number    string
	Kind      string
	Status    string
	Currency  Currency
	IssueDate string
	Total     Money
	Paid      Money
	Balance   Money
}

func (c Customer) Validate() error {
	kind := c.Type
	if kind == "" {
		kind = CustomerTypeIndividual
	}
	if kind != CustomerTypeIndividual && kind != CustomerTypeCompany {
		return errors.New("نوع مشتری باید حقیقی یا حقوقی باشد")
	}
	if kind == CustomerTypeCompany {
		if strings.TrimSpace(c.LegalName) == "" && strings.TrimSpace(c.Name) == "" {
			return errors.New("نام شرکت الزامی است")
		}
	} else {
		first, last := strings.TrimSpace(c.FirstName), strings.TrimSpace(c.LastName)
		if first == "" && last == "" && strings.TrimSpace(c.Name) == "" {
			return errors.New("نام و نام خانوادگی مشتری الزامی است")
		}
		if (first == "") != (last == "") {
			return errors.New("برای مشتری حقیقی، نام و نام خانوادگی هر دو الزامی هستند")
		}
	}
	if len(c.Name) > 300 || len(c.FirstName) > 150 || len(c.LastName) > 150 ||
		len(c.LegalName) > 300 || len(c.TaxID) > 100 || len(c.Phone) > 80 ||
		len(c.Email) > 254 || len(c.Address) > 2000 {
		return errors.New("طول یکی از اطلاعات مشتری بیش از حد مجاز است")
	}
	if len(c.Phones) > 30 || len(c.Addresses) > 30 {
		return errors.New("تعداد شماره تماس یا نشانی بیش از حد مجاز است")
	}
	for _, phone := range c.Phones {
		if len(strings.TrimSpace(phone)) > 80 {
			return errors.New("شماره تماس بیش از حد مجاز است")
		}
	}
	for _, address := range c.Addresses {
		if len(strings.TrimSpace(address)) > 2000 {
			return errors.New("نشانی بیش از حد مجاز است")
		}
	}
	return nil
}
