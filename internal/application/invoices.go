package application

import (
	"context"
	"errors"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

type InvoiceWorkflowStore interface {
	CreateInvoiceDraft(context.Context, domain.InvoiceDocument) (int64, string, error)
	IssueInvoice(context.Context, int64) error
	RegisterPayment(context.Context, domain.Payment, []domain.PaymentAllocation) (int64, error)
	InvoiceBalance(context.Context, int64) (domain.Money, error)
}

// Invoices coordinates invoice lifecycle and payment operations. Financial rules
// remain in the domain layer; persistence and transaction boundaries remain in SQLite.
type InvoiceQueryStore interface {
	ListInvoiceDocuments(context.Context, string) ([]domain.InvoiceListItem, error)
	GetInvoiceDocument(context.Context, int64) (domain.InvoiceDocument, error)
	ConvertProforma(context.Context, int64) error
	UpdateInvoiceDraft(context.Context, domain.InvoiceDocument) error
}

type InvoicePaymentHistoryStore interface {
	ListPaymentHistory(context.Context) ([]domain.PaymentHistoryEntry, error)
}

type Invoices struct{ store InvoiceWorkflowStore }

func NewInvoices(store InvoiceWorkflowStore) *Invoices { return &Invoices{store: store} }

func (s *Invoices) CreateDraft(ctx context.Context, draft domain.InvoiceDocument) (int64, string, error) {
	if err := draft.Validate(); err != nil {
		return 0, "", err
	}
	return s.store.CreateInvoiceDraft(ctx, draft)
}

func (s *Invoices) Issue(ctx context.Context, id int64) error {
	return s.store.IssueInvoice(ctx, id)
}

func (s *Invoices) RegisterPayment(ctx context.Context, payment domain.Payment, allocations []domain.PaymentAllocation) (int64, error) {
	if err := domain.ValidateAllocations(payment, allocations); err != nil {
		return 0, err
	}
	return s.store.RegisterPayment(ctx, payment, allocations)
}

func (s *Invoices) Balance(ctx context.Context, invoiceID int64) (domain.Money, error) {
	return s.store.InvoiceBalance(ctx, invoiceID)
}

func (s *Invoices) PaymentHistory(ctx context.Context) ([]domain.PaymentHistoryEntry, error) {
	store, ok := s.store.(InvoicePaymentHistoryStore)
	if !ok {
		return nil, errors.New("مخزن فاکتور از دفتر دریافت‌ها پشتیبانی نمی‌کند")
	}
	return store.ListPaymentHistory(ctx)
}

func (s *Invoices) List(ctx context.Context, query string) ([]domain.InvoiceListItem, error) {
	store, ok := s.store.(InvoiceQueryStore)
	if !ok {
		return nil, errors.New("مخزن فاکتور از فهرست اسناد پشتیبانی نمی‌کند")
	}
	return store.ListInvoiceDocuments(ctx, query)
}

func (s *Invoices) Get(ctx context.Context, id int64) (domain.InvoiceDocument, error) {
	store, ok := s.store.(InvoiceQueryStore)
	if !ok {
		return domain.InvoiceDocument{}, errors.New("مخزن فاکتور از خواندن سند پشتیبانی نمی‌کند")
	}
	return store.GetInvoiceDocument(ctx, id)
}

func (s *Invoices) ConvertProforma(ctx context.Context, id int64) error {
	store, ok := s.store.(InvoiceQueryStore)
	if !ok {
		return errors.New("مخزن فاکتور از تبدیل پیش‌فاکتور پشتیبانی نمی‌کند")
	}
	return store.ConvertProforma(ctx, id)
}

func (s *Invoices) UpdateDraft(ctx context.Context, doc domain.InvoiceDocument) error {
	if doc.ID <= 0 {
		return errors.New("شناسه سند برای ویرایش نامعتبر است")
	}
	if err := doc.Validate(); err != nil {
		return err
	}
	store, ok := s.store.(InvoiceQueryStore)
	if !ok {
		return errors.New("مخزن فاکتور از ویرایش پیش‌نویس پشتیبانی نمی‌کند")
	}
	return store.UpdateInvoiceDraft(ctx, doc)
}
