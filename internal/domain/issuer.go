package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	IssuerTypeIndividual = "individual"
	IssuerTypeCompany    = "company"
)

type IssuerBankDetails struct {
	ID            int64
	BankName      string
	CardNumber    string
	AccountNumber string
	IBAN          string
}

type IssuerProfile struct {
	ID          int64
	Type        string
	Name        string
	FirstName   string
	LastName    string
	LegalName   string
	TaxID       string
	Phone       string // Legacy primary phone retained for compatibility.
	Phones      []string
	Email       string
	Address     string
	BankDetails []IssuerBankDetails
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p IssuerProfile) Validate() error {
	if p.Type == "" {
		p.Type = IssuerTypeIndividual
	}
	if p.Type != IssuerTypeIndividual && p.Type != IssuerTypeCompany {
		return errors.New("نوع صادرکننده باید حقیقی یا حقوقی باشد")
	}
	if p.Type == IssuerTypeIndividual &&
		strings.TrimSpace(p.FirstName)+strings.TrimSpace(p.LastName) == "" &&
		strings.TrimSpace(p.Name) == "" {
		return errors.New("نام و نام خانوادگی صادرکننده حقیقی الزامی است")
	}
	if p.Type == IssuerTypeCompany && strings.TrimSpace(p.LegalName) == "" {
		return errors.New("نام شرکت الزامی است")
	}
	if len(p.Name) > 200 || len(p.FirstName) > 150 || len(p.LastName) > 150 ||
		len(p.LegalName) > 300 || (p.Type == IssuerTypeCompany && len(p.LegalName) > 200) ||
		len(p.TaxID) > 100 || len(p.Phone) > 80 || len(p.Email) > 254 || len(p.Address) > 2000 {
		return errors.New("طول یکی از اطلاعات صادرکننده بیش از حد مجاز است")
	}
	if len(p.Phones) > 30 || len(p.BankDetails) > 30 {
		return errors.New("تعداد شماره‌های تماس یا حساب‌های بانکی بیش از حد مجاز است")
	}
	for _, phone := range p.Phones {
		if len(strings.TrimSpace(phone)) > 80 {
			return errors.New("شماره تماس بیش از حد مجاز است")
		}
	}
	for _, account := range p.BankDetails {
		if len(account.CardNumber) > 100 || len(account.AccountNumber) > 100 || len(account.IBAN) > 100 {
			return errors.New("اطلاعات حساب بانکی بیش از حد مجاز است")
		}
		if strings.TrimSpace(account.CardNumber) == "" &&
			strings.TrimSpace(account.AccountNumber) == "" &&
			strings.TrimSpace(account.IBAN) == "" {
			return errors.New("هر ردیف حساب باید دست‌کم یک شماره کارت، حساب یا شبا داشته باشد")
		}
	}
	return nil
}

type Setting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

func (s Setting) Validate() error {
	if strings.TrimSpace(s.Key) == "" {
		return errors.New("کلید تنظیمات الزامی است")
	}
	if len(s.Key) > 200 || len(s.Value) > 10000 {
		return errors.New("طول تنظیمات بیش از حد مجاز است")
	}
	return nil
}
