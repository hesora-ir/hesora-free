package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func (s *Store) SaveExpense(ctx context.Context, expense domain.Expense) (int64, error) {
	if err := expense.Validate(); err != nil {
		return 0, err
	}
	date := expense.Date.UTC().Format("2006-01-02")
	if expense.ID == 0 {
		result, err := s.db.ExecContext(ctx, `INSERT INTO expenses(amount_minor,currency,expense_date,category,note,created_at) VALUES(?,?,?,?,?,?)`, expense.Amount, string(expense.Currency), date, expense.Category, expense.Note, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return 0, err
		}
		return result.LastInsertId()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE expenses SET amount_minor=?,currency=?,expense_date=?,category=?,note=? WHERE id=?`, expense.Amount, string(expense.Currency), date, expense.Category, expense.Note, expense.ID)
	return expense.ID, err
}

func (s *Store) ListExpenses(ctx context.Context) ([]domain.Expense, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,amount_minor,currency,expense_date,category,note,created_at FROM expenses ORDER BY expense_date DESC,id DESC LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Expense
	for rows.Next() {
		var e domain.Expense
		var currency, date, created string
		if err = rows.Scan(&e.ID, &e.Amount, &currency, &date, &e.Category, &e.Note, &created); err != nil {
			return nil, err
		}
		e.Currency = domain.Currency(currency)
		e.Date, _ = time.Parse("2006-01-02", date)
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) DeleteExpense(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM expenses WHERE id=?`, id)
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}
