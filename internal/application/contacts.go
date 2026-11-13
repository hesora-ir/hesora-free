package application

import (
	"context"
	"strings"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

type ContactStore interface {
	CreateCustomer(context.Context, domain.Customer) (int64, error)
	UpdateCustomer(context.Context, domain.Customer) error
	ListCustomers(context.Context, string) ([]domain.Customer, error)
	CustomerInvoices(context.Context, int64) ([]domain.CustomerInvoice, error)
	SaveIssuerProfile(context.Context, domain.IssuerProfile) (int64, error)
	ListIssuerProfiles(context.Context) ([]domain.IssuerProfile, error)
	SetSetting(context.Context, domain.Setting) error
	GetSetting(context.Context, string) (domain.Setting, error)
}

type Contacts struct {
	store ContactStore
}

func NewContacts(store ContactStore) *Contacts {
	return &Contacts{store: store}
}

func (c *Contacts) CreateCustomer(ctx context.Context, customer domain.Customer) (int64, error) {
	customer.Name = strings.TrimSpace(customer.Name)
	if err := customer.Validate(); err != nil {
		return 0, err
	}
	return c.store.CreateCustomer(ctx, customer)
}

func (c *Contacts) UpdateCustomer(ctx context.Context, customer domain.Customer) error {
	customer.Name = strings.TrimSpace(customer.Name)
	if err := customer.Validate(); err != nil {
		return err
	}
	return c.store.UpdateCustomer(ctx, customer)
}

func (c *Contacts) SearchCustomers(ctx context.Context, query string) ([]domain.Customer, error) {
	return c.store.ListCustomers(ctx, strings.TrimSpace(query))
}

func (c *Contacts) CustomerInvoices(ctx context.Context, customerID int64) ([]domain.CustomerInvoice, error) {
	return c.store.CustomerInvoices(ctx, customerID)
}

func (c *Contacts) SaveIssuerProfile(ctx context.Context, profile domain.IssuerProfile) (int64, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	if err := profile.Validate(); err != nil {
		return 0, err
	}
	return c.store.SaveIssuerProfile(ctx, profile)
}

func (c *Contacts) IssuerProfiles(ctx context.Context) ([]domain.IssuerProfile, error) {
	return c.store.ListIssuerProfiles(ctx)
}

func (c *Contacts) SetSetting(ctx context.Context, setting domain.Setting) error {
	return c.store.SetSetting(ctx, setting)
}

func (c *Contacts) Setting(ctx context.Context, key string) (domain.Setting, error) {
	return c.store.GetSetting(ctx, key)
}
