package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func (s *Store) RegisterPayment(ctx context.Context, payment domain.Payment, allocations []domain.PaymentAllocation) (int64, error) {
	if err := domain.ValidateAllocations(payment, allocations); err != nil {
		return 0, err
	}
	payment.ExternalRef = strings.TrimSpace(payment.ExternalRef)
	payment.Method = strings.TrimSpace(payment.Method)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("شروع تراکنش پرداخت: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO payments(external_ref,amount_minor,currency,paid_at,method,note,created_at)
		VALUES(?,?,?,?,?,?,?)`, payment.ExternalRef, int64(payment.Amount), string(payment.Currency),
		payment.PaidAt.UTC().Format(time.RFC3339Nano), payment.Method, payment.Note, now)
	if err != nil {
		return 0, fmt.Errorf("ثبت پرداخت (احتمالاً شناسه مرجع تکراری است): %w", err)
	}
	paymentID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, allocation := range allocations {
		var status, currency string
		var total int64
		err := tx.QueryRowContext(ctx, "SELECT status,currency,total_minor+additional_charge_minor FROM invoices WHERE id=?", allocation.InvoiceID).Scan(&status, &currency, &total)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("فاکتور %d پیدا نشد", allocation.InvoiceID)
		}
		if err != nil {
			return 0, err
		}
		if status != string(domain.InvoiceIssued) {
			return 0, fmt.Errorf("فاکتور %d صادر نشده یا باطل است", allocation.InvoiceID)
		}
		if currency != string(payment.Currency) {
			return 0, fmt.Errorf("واحد پول پرداخت با فاکتور %d یکسان نیست", allocation.InvoiceID)
		}
		var allocated int64
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_minor),0) FROM payment_allocations WHERE invoice_id=?", allocation.InvoiceID).Scan(&allocated); err != nil {
			return 0, err
		}
		if allocated > total || int64(allocation.Amount) > total-allocated {
			return 0, fmt.Errorf("مبلغ تخصیص از مانده فاکتور %d بیشتر است", allocation.InvoiceID)
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO payment_allocations(payment_id,invoice_id,amount_minor,created_at) VALUES(?,?,?,?)",
			paymentID, allocation.InvoiceID, int64(allocation.Amount), now)
		if err != nil {
			return 0, fmt.Errorf("تخصیص پرداخت به فاکتور %d: %w", allocation.InvoiceID, err)
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('invoice',?,'payment_allocated',?,'{}')", allocation.InvoiceID, now)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ثبت تراکنش پرداخت: %w", err)
	}
	return paymentID, nil
}

func (s *Store) InvoiceBalance(ctx context.Context, invoiceID int64) (domain.Money, error) {
	if invoiceID <= 0 {
		return 0, errors.New("شناسه فاکتور نامعتبر است")
	}
	var total, paid int64
	err := s.db.QueryRowContext(ctx, "SELECT total_minor+additional_charge_minor, (SELECT COALESCE(SUM(amount_minor),0) FROM payment_allocations WHERE invoice_id=invoices.id) FROM invoices WHERE id=?", invoiceID).Scan(&total, &paid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("فاکتور پیدا نشد")
	}
	if err != nil {
		return 0, err
	}
	if paid > total {
		return 0, errors.New("مجموع پرداخت‌های تخصیص‌یافته از مبلغ فاکتور بیشتر است")
	}
	return domain.Money(total - paid), nil
}
