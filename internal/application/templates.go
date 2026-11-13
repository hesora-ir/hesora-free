package application

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/hesora-ir/hesora-free/internal/domain"
	"github.com/hesora-ir/hesora-free/internal/licensing"
	invoicehtml "github.com/hesora-ir/hesora-free/internal/render/html"
)

type TemplateStore interface {
	SaveLetterhead(context.Context, domain.Letterhead) (int64, error)
	ListLetterheads(context.Context) ([]domain.Letterhead, error)
	SaveBankAccount(context.Context, domain.BankAccount) (int64, error)
	ListBankAccounts(context.Context, int64) ([]domain.BankAccount, error)
	BankAccountByIDs(context.Context, []int64) ([]domain.BankAccount, error)
	SaveDocumentTemplate(context.Context, domain.DocumentTemplate) (int64, error)
	ListDocumentTemplates(context.Context) ([]domain.DocumentTemplate, error)
}

type Templates struct {
	store      TemplateStore
	proEnabled atomic.Bool
}

func NewTemplates(store TemplateStore) *Templates { return &Templates{store: store} }
func (s *Templates) SaveLetterhead(ctx context.Context, h domain.Letterhead) (int64, error) {
	if err := h.Validate(); err != nil {
		return 0, err
	}
	return s.store.SaveLetterhead(ctx, h)
}
func (s *Templates) Letterheads(ctx context.Context) ([]domain.Letterhead, error) {
	return s.store.ListLetterheads(ctx)
}
func (s *Templates) SaveBankAccount(ctx context.Context, a domain.BankAccount) (int64, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	return s.store.SaveBankAccount(ctx, a)
}
func (s *Templates) BankAccounts(ctx context.Context, issuerID int64) ([]domain.BankAccount, error) {
	return s.store.ListBankAccounts(ctx, issuerID)
}
func (s *Templates) SaveTemplate(ctx context.Context, t domain.DocumentTemplate) (int64, error) {
	if !s.proEnabled.Load() {
		return 0, licensing.ErrFeatureNotLicensed
	}
	if err := t.Validate(); err != nil {
		return 0, err
	}
	return s.store.SaveDocumentTemplate(ctx, t)
}
func (s *Templates) SetProEnabled(enabled bool) { s.proEnabled.Store(enabled) }
func (s *Templates) ProEnabled() bool           { return s.proEnabled.Load() }
func (s *Templates) DocumentTemplates(ctx context.Context) ([]domain.DocumentTemplate, error) {
	return s.store.ListDocumentTemplates(ctx)
}

func (s *Templates) PreviewInvoice(ctx context.Context, doc domain.InvoiceDocument, t domain.DocumentTemplate) (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	if len(doc.Items) == 0 {
		return "", errors.New("برای پیش‌نمایش سند باید حداقل یک قلم داشته باشد")
	}
	totals, err := domain.CalculateFinancialsWithCharges(doc.Items, doc.InvoiceDiscount, doc.TaxRateBPS, doc.AdditionalCharges)
	if err != nil {
		return "", err
	}
	data := invoicehtml.PreviewData{Document: doc, Totals: totals, Template: t}
	if t.LetterheadID > 0 {
		headers, err := s.store.ListLetterheads(ctx)
		if err != nil {
			return "", err
		}
		found := false
		for _, h := range headers {
			if h.ID == t.LetterheadID {
				data.Letterhead = h
				found = true
				break
			}
		}
		if !found {
			return "", errors.New("سربرگ انتخاب‌شده پیدا نشد")
		}
	}
	if t.ShowBankAccounts && len(t.BankAccountIDs) > 0 {
		data.BankAccounts, err = s.store.BankAccountByIDs(ctx, t.BankAccountIDs)
		if err != nil {
			return "", err
		}
	}
	return invoicehtml.RenderInvoicePreview(data)
}
