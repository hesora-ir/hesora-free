package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

// Dashboard reads all counters and events from one consistent database snapshot.
// Aggregate allocations before joining invoices to avoid counting an invoice twice.
func (s *Store) Dashboard(ctx context.Context) (domain.DashboardSnapshot, error) {
	var result domain.DashboardSnapshot
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM customers").Scan(&result.Customers); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT i.currency,
		COALESCE(SUM(CASE WHEN i.total_minor+i.additional_charge_minor > COALESCE(p.paid,0) THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN i.total_minor+i.additional_charge_minor = COALESCE(p.paid,0) THEN 1 ELSE 0 END),0),
		COALESCE(SUM(i.total_minor+i.additional_charge_minor - COALESCE(p.paid,0)),0)
		FROM invoices i LEFT JOIN (
			SELECT pa.invoice_id, SUM(pa.amount_minor) AS paid FROM payment_allocations pa LEFT JOIN cloud_payment_reversals r ON r.local_payment_id=pa.payment_id WHERE r.local_payment_id IS NULL GROUP BY pa.invoice_id
		) p ON p.invoice_id=i.id
		WHERE i.kind='invoice' AND i.status='issued' GROUP BY i.currency`)
	if err != nil {
		return result, fmt.Errorf("خواندن مانده‌ها: %w", err)
	}
	for rows.Next() {
		var currency string
		var unsettled, settled int64
		var outstanding domain.Money
		if err := rows.Scan(&currency, &unsettled, &settled, &outstanding); err != nil {
			rows.Close()
			return result, err
		}
		result.Unsettled += unsettled
		result.Settled += settled
		if currency == "IRR" {
			result.Rial.Outstanding = outstanding
		} else {
			result.Toman.Outstanding = outstanding
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN p.currency='IRT' THEN p.amount_minor ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN p.currency='IRR' THEN p.amount_minor ELSE 0 END),0) FROM payments p
		LEFT JOIN cloud_payment_reversals r ON r.local_payment_id=p.id WHERE r.local_payment_id IS NULL`).Scan(&result.Toman.Received, &result.Rial.Received); err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT i.number, a.action, a.occurred_at
		FROM audit_events a JOIN invoices i ON a.entity_type='invoice' AND a.entity_id=i.id
		WHERE a.action IN ('created','issued','draft_updated','proforma_converted','payment_allocated')
		ORDER BY a.occurred_at DESC, a.id DESC LIMIT 8`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var event domain.DashboardActivity
		var occurred string
		if err := rows.Scan(&event.Number, &event.Action, &occurred); err != nil {
			rows.Close()
			return result, err
		}
		event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurred)
		if err != nil {
			rows.Close()
			return result, err
		}
		result.Activities = append(result.Activities, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}
