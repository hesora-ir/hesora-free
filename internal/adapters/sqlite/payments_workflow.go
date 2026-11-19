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
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_minor),0) FROM payment_allocations pa WHERE invoice_id=? AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)", allocation.InvoiceID).Scan(&allocated); err != nil {
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
	err := s.db.QueryRowContext(ctx, "SELECT total_minor+additional_charge_minor, (SELECT COALESCE(SUM(amount_minor),0) FROM payment_allocations pa WHERE pa.invoice_id=invoices.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)) FROM invoices WHERE id=?", invoiceID).Scan(&total, &paid)
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

func (s *Store) ListPaymentHistory(ctx context.Context) ([]domain.PaymentHistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.external_ref,p.amount_minor,p.currency,p.paid_at,p.method,p.note,
		r.reversed_at,r.reason,pa.invoice_id,i.number,pa.amount_minor
		FROM payments p
		LEFT JOIN cloud_payment_reversals r ON r.local_payment_id=p.id
		LEFT JOIN payment_allocations pa ON pa.payment_id=p.id
		LEFT JOIN invoices i ON i.id=pa.invoice_id
		ORDER BY p.id DESC,pa.invoice_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]domain.PaymentHistoryEntry, 0)
	positions := make(map[int64]int)
	for rows.Next() {
		var id, amount int64
		var reference, currency, paidAt, method, note string
		var reversedAt sql.NullInt64
		var reversalReason sql.NullString
		var invoiceID sql.NullInt64
		var invoiceNumber sql.NullString
		var allocationAmount sql.NullInt64
		if err := rows.Scan(&id, &reference, &amount, &currency, &paidAt, &method, &note, &reversedAt, &reversalReason, &invoiceID, &invoiceNumber, &allocationAmount); err != nil {
			return nil, err
		}
		position, exists := positions[id]
		if !exists {
			date, parseErr := time.Parse(time.RFC3339Nano, paidAt)
			if parseErr != nil {
				date, parseErr = time.Parse("2006-01-02", paidAt)
			}
			if parseErr != nil {
				return nil, fmt.Errorf("parse payment %d date: %w", id, parseErr)
			}
			entry := domain.PaymentHistoryEntry{Payment: domain.Payment{ID: id, ExternalRef: reference, Amount: domain.Money(amount), Currency: domain.Currency(currency), PaidAt: date, Method: method, Note: note}}
			if reversedAt.Valid {
				entry.ReversedAt = time.Unix(reversedAt.Int64, 0).UTC()
			}
			if reversalReason.Valid {
				entry.ReversalReason = reversalReason.String
			}
			entries = append(entries, entry)
			position = len(entries) - 1
			positions[id] = position
		}
		if invoiceID.Valid {
			entries[position].Allocations = append(entries[position].Allocations, domain.PaymentHistoryAllocation{InvoiceID: invoiceID.Int64, InvoiceNumber: invoiceNumber.String, Amount: domain.Money(allocationAmount.Int64)})
		}
	}
	return entries, rows.Err()
}
