package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
	"github.com/hesora-ir/hesora-free/internal/licensing"
)

func normalizeCustomer(c domain.Customer) domain.Customer {
	c.Type = strings.TrimSpace(c.Type)
	if c.Type == "" {
		c.Type = domain.CustomerTypeIndividual
	}
	c.Name = strings.TrimSpace(c.Name)
	c.FirstName = strings.TrimSpace(c.FirstName)
	c.LastName = strings.TrimSpace(c.LastName)
	c.LegalName = strings.TrimSpace(c.LegalName)
	c.TaxID = strings.TrimSpace(c.TaxID)
	if c.Type == domain.CustomerTypeCompany {
		if c.LegalName == "" {
			c.LegalName = c.Name
		}
		if c.LegalName != "" {
			c.Name = c.LegalName
		}
	} else if c.FirstName != "" || c.LastName != "" {
		c.Name = strings.TrimSpace(c.FirstName + " " + c.LastName)
	}
	if len(c.Phones) == 0 && strings.TrimSpace(c.Phone) != "" {
		c.Phones = []string{strings.TrimSpace(c.Phone)}
	}
	if len(c.Addresses) == 0 && strings.TrimSpace(c.Address) != "" {
		c.Addresses = []string{strings.TrimSpace(c.Address)}
	}
	c.Phones = normalizeStrings(c.Phones)
	c.Addresses = normalizeStrings(c.Addresses)
	c.Phone, c.Address = "", ""
	if len(c.Phones) > 0 {
		c.Phone = c.Phones[0]
	}
	if len(c.Addresses) > 0 {
		c.Address = c.Addresses[0]
	}
	return c
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *Store) CreateCustomer(ctx context.Context, customer domain.Customer) (int64, error) {
	customer = normalizeCustomer(customer)
	if err := customer.Validate(); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("شروع تراکنش مشتری: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var customerCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM customers").Scan(&customerCount); err != nil {
		return 0, fmt.Errorf("بررسی سقف مشتریان: %w", err)
	}
	if !s.ProEnabled() && customerCount >= licensing.FreeCustomerLimit {
		return 0, fmt.Errorf("نسخهٔ رایگان حداکثر %d مشتری می‌پذیرد", licensing.FreeCustomerLimit)
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO customers(customer_type,first_name,last_name,legal_name,tax_id,name,phone,email,address,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)",
		customer.Type, customer.FirstName, customer.LastName, customer.LegalName, customer.TaxID,
		customer.Name, customer.Phone, strings.TrimSpace(customer.Email), customer.Address, now, now)
	if err != nil {
		return 0, fmt.Errorf("ذخیره مشتری: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("دریافت شناسه مشتری: %w", err)
	}
	if err := replaceCustomerChildren(ctx, tx, id, customer); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ثبت مشتری: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateCustomer(ctx context.Context, customer domain.Customer) error {
	customer = normalizeCustomer(customer)
	if customer.ID <= 0 {
		return errors.New("شناسه مشتری نامعتبر است")
	}
	if err := customer.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("شروع تراکنش ویرایش مشتری: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, "UPDATE customers SET customer_type=?,first_name=?,last_name=?,legal_name=?,tax_id=?,name=?,phone=?,email=?,address=?,updated_at=? WHERE id=?",
		customer.Type, customer.FirstName, customer.LastName, customer.LegalName, customer.TaxID,
		customer.Name, customer.Phone, strings.TrimSpace(customer.Email), customer.Address,
		time.Now().UTC().Format(time.RFC3339Nano), customer.ID)
	if err != nil {
		return fmt.Errorf("ویرایش مشتری: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("مشتری پیدا نشد")
	}
	if err := replaceCustomerChildren(ctx, tx, customer.ID, customer); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ثبت تغییرات مشتری: %w", err)
	}
	return nil
}

func replaceCustomerChildren(ctx context.Context, tx *sql.Tx, id int64, c domain.Customer) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM customer_phones WHERE customer_id=?", id); err != nil {
		return fmt.Errorf("به‌روزرسانی شماره‌های مشتری: %w", err)
	}
	for i, phone := range c.Phones {
		if _, err := tx.ExecContext(ctx, "INSERT INTO customer_phones(customer_id,phone,position) VALUES(?,?,?)", id, phone, i); err != nil {
			return fmt.Errorf("ذخیره شماره تماس مشتری: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM customer_addresses WHERE customer_id=?", id); err != nil {
		return fmt.Errorf("به‌روزرسانی نشانی‌های مشتری: %w", err)
	}
	for i, address := range c.Addresses {
		if _, err := tx.ExecContext(ctx, "INSERT INTO customer_addresses(customer_id,address,position) VALUES(?,?,?)", id, address, i); err != nil {
			return fmt.Errorf("ذخیره نشانی مشتری: %w", err)
		}
	}
	return nil
}

func (s *Store) CustomerByID(ctx context.Context, id int64) (domain.Customer, error) {
	if id <= 0 {
		return domain.Customer{}, errors.New("شناسه مشتری نامعتبر است")
	}
	var c domain.Customer
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, "SELECT id,customer_type,name,first_name,last_name,legal_name,tax_id,phone,email,address,created_at,updated_at FROM customers WHERE id=?", id).Scan(
		&c.ID, &c.Type, &c.Name, &c.FirstName, &c.LastName, &c.LegalName, &c.TaxID, &c.Phone, &c.Email, &c.Address, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Customer{}, errors.New("مشتری پیدا نشد")
	}
	if err != nil {
		return domain.Customer{}, fmt.Errorf("خواندن مشتری: %w", err)
	}
	c.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.Customer{}, fmt.Errorf("تاریخ ایجاد مشتری نامعتبر است: %w", err)
	}
	c.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return domain.Customer{}, fmt.Errorf("تاریخ ویرایش مشتری نامعتبر است: %w", err)
	}
	if err := s.loadCustomerChildren(ctx, &c); err != nil {
		return domain.Customer{}, err
	}
	return c, nil
}

func (s *Store) ListCustomers(ctx context.Context, query string) ([]domain.Customer, error) {
	query = strings.TrimSpace(query)
	rows, err := s.db.QueryContext(ctx, "SELECT c.id,c.customer_type,c.name,c.first_name,c.last_name,c.legal_name,c.tax_id,c.phone,c.email,c.address,c.created_at,c.updated_at,"+
		"(SELECT COUNT(*) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued'),"+
		"(SELECT COUNT(*) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0)<i.total_minor+i.additional_charge_minor),"+
		"(SELECT COUNT(*) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0)>=i.total_minor+i.additional_charge_minor),"+
		"(SELECT COUNT(*) FROM invoices i WHERE i.customer_id=c.id AND i.kind='proforma' AND i.status<>'void'),"+
		"(SELECT COALESCE(SUM(i.total_minor+i.additional_charge_minor),0) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND i.currency='IRR'),"+
		"(SELECT COALESCE(SUM(i.total_minor+i.additional_charge_minor),0) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND i.currency='IRT'),"+
		"(SELECT COALESCE(SUM(MAX(i.total_minor+i.additional_charge_minor-COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0),0)),0) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND i.currency='IRR'),"+
		"(SELECT COALESCE(SUM(MAX(i.total_minor+i.additional_charge_minor-COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0),0)),0) FROM invoices i WHERE i.customer_id=c.id AND i.kind='invoice' AND i.status='issued' AND i.currency='IRT')"+
		" FROM customers c WHERE ?='' OR c.name LIKE '%'||?||'%' OR c.phone LIKE '%'||?||'%' OR c.email LIKE '%'||?||'%' OR c.tax_id LIKE '%'||?||'%' OR EXISTS(SELECT 1 FROM customer_phones cp WHERE cp.customer_id=c.id AND cp.phone LIKE '%'||?||'%') ORDER BY c.name COLLATE NOCASE,c.id DESC LIMIT 500",
		query, query, query, query, query, query)
	if err != nil {
		return nil, fmt.Errorf("جست‌وجوی مشتریان: %w", err)
	}
	defer rows.Close()
	customers := make([]domain.Customer, 0)
	for rows.Next() {
		var c domain.Customer
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.FirstName, &c.LastName, &c.LegalName, &c.TaxID, &c.Phone, &c.Email, &c.Address, &createdAt, &updatedAt,
			&c.IssuedInvoiceCount, &c.OpenInvoiceCount, &c.SettledInvoiceCount, &c.ProformaCount, &c.IssuedTotalIRR, &c.IssuedTotalIRT, &c.DebtIRR, &c.DebtIRT); err != nil {
			return nil, err
		}
		c.InvoiceCount = c.IssuedInvoiceCount
		c.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		c.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// Release the result set before querying child tables. With a one-connection
	// SQLite pool, querying while rows is still open can deadlock indefinitely.
	for i := range customers {
		if err := s.loadCustomerChildren(ctx, &customers[i]); err != nil {
			return nil, err
		}
	}
	return customers, nil
}

func (s *Store) loadCustomerChildren(ctx context.Context, c *domain.Customer) error {
	phoneRows, err := s.db.QueryContext(ctx, "SELECT phone FROM customer_phones WHERE customer_id=? ORDER BY position,id", c.ID)
	if err != nil {
		return err
	}
	for phoneRows.Next() {
		var phone string
		if err := phoneRows.Scan(&phone); err != nil {
			phoneRows.Close()
			return err
		}
		c.Phones = append(c.Phones, phone)
	}
	if err := phoneRows.Err(); err != nil {
		phoneRows.Close()
		return err
	}
	if err := phoneRows.Close(); err != nil {
		return err
	}
	if len(c.Phones) == 0 && strings.TrimSpace(c.Phone) != "" {
		c.Phones = []string{c.Phone}
	}
	addressRows, err := s.db.QueryContext(ctx, "SELECT address FROM customer_addresses WHERE customer_id=? ORDER BY position,id", c.ID)
	if err != nil {
		return err
	}
	for addressRows.Next() {
		var address string
		if err := addressRows.Scan(&address); err != nil {
			addressRows.Close()
			return err
		}
		c.Addresses = append(c.Addresses, address)
	}
	if err := addressRows.Err(); err != nil {
		addressRows.Close()
		return err
	}
	if err := addressRows.Close(); err != nil {
		return err
	}
	if len(c.Addresses) == 0 && strings.TrimSpace(c.Address) != "" {
		c.Addresses = []string{c.Address}
	}
	return nil
}

func (s *Store) CustomerInvoices(ctx context.Context, customerID int64) ([]domain.CustomerInvoice, error) {
	if customerID <= 0 {
		return nil, errors.New("شناسه مشتری نامعتبر است")
	}
	rows, err := s.db.QueryContext(ctx, "SELECT i.id,i.number,i.kind,i.status,i.currency,i.issue_date,i.total_minor+i.additional_charge_minor,COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0) FROM invoices i WHERE i.customer_id=? AND i.status<>'void' AND i.kind IN ('invoice','proforma') ORDER BY CASE i.kind WHEN 'proforma' THEN 0 ELSE 1 END,i.issue_date DESC,i.id DESC", customerID)
	if err != nil {
		return nil, fmt.Errorf("خواندن فاکتورهای مشتری: %w", err)
	}
	defer rows.Close()
	result := make([]domain.CustomerInvoice, 0)
	for rows.Next() {
		var item domain.CustomerInvoice
		var total, paid int64
		if err := rows.Scan(&item.ID, &item.Number, &item.Kind, &item.Status, &item.Currency, &item.IssueDate, &total, &paid); err != nil {
			return nil, err
		}
		item.Total, item.Paid = domain.Money(total), domain.Money(paid)
		item.Balance = domain.Money(total - paid)
		if item.Balance < 0 {
			item.Balance = 0
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
