package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func (s *Store) SaveLetterhead(ctx context.Context, h domain.Letterhead) (int64, error) {
	if err := h.Validate(); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var issuer any
	if h.IssuerProfileID > 0 {
		issuer = h.IssuerProfileID
	}
	if h.ID == 0 {
		r, err := s.db.ExecContext(ctx, `INSERT INTO issuer_letterheads(issuer_profile_id,name,logo_path,title,subtitle,contact_line,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, issuer, h.Name, h.LogoPath, h.Title, h.Subtitle, h.ContactLine, now, now)
		if err != nil {
			return 0, fmt.Errorf("ذخیره سربرگ: %w", err)
		}
		return r.LastInsertId()
	}
	r, err := s.db.ExecContext(ctx, `UPDATE issuer_letterheads SET issuer_profile_id=?,name=?,logo_path=?,title=?,subtitle=?,contact_line=?,updated_at=? WHERE id=?`, issuer, h.Name, h.LogoPath, h.Title, h.Subtitle, h.ContactLine, now, h.ID)
	if err != nil {
		return 0, fmt.Errorf("ویرایش سربرگ: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errors.New("سربرگ پیدا نشد")
	}
	return h.ID, nil
}

func (s *Store) ListLetterheads(ctx context.Context) ([]domain.Letterhead, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,COALESCE(issuer_profile_id,0),name,logo_path,title,subtitle,contact_line,created_at,updated_at FROM issuer_letterheads ORDER BY id DESC LIMIT 500`)
	if err != nil {
		return nil, fmt.Errorf("خواندن سربرگ‌ها: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Letterhead, 0)
	for rows.Next() {
		var v domain.Letterhead
		var c, u string
		if err := rows.Scan(&v.ID, &v.IssuerProfileID, &v.Name, &v.LogoPath, &v.Title, &v.Subtitle, &v.ContactLine, &c, &u); err != nil {
			return nil, err
		}
		v.CreatedAt, err = time.Parse(time.RFC3339Nano, c)
		if err != nil {
			return nil, err
		}
		v.UpdatedAt, err = time.Parse(time.RFC3339Nano, u)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SaveBankAccount(ctx context.Context, a domain.BankAccount) (int64, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var issuer any
	if a.IssuerProfileID > 0 {
		issuer = a.IssuerProfileID
	}
	if a.ID == 0 {
		r, err := s.db.ExecContext(ctx, `INSERT INTO bank_accounts(issuer_profile_id,bank_name,account_holder,account_number,card_number,iban,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, issuer, a.BankName, a.AccountHolder, a.AccountNumber, a.CardNumber, a.IBAN, now, now)
		if err != nil {
			return 0, fmt.Errorf("ذخیره حساب بانکی: %w", err)
		}
		return r.LastInsertId()
	}
	r, err := s.db.ExecContext(ctx, `UPDATE bank_accounts SET issuer_profile_id=?,bank_name=?,account_holder=?,account_number=?,card_number=?,iban=?,updated_at=? WHERE id=?`, issuer, a.BankName, a.AccountHolder, a.AccountNumber, a.CardNumber, a.IBAN, now, a.ID)
	if err != nil {
		return 0, fmt.Errorf("ویرایش حساب بانکی: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, errors.New("حساب بانکی پیدا نشد")
	}
	return a.ID, nil
}

func (s *Store) ListBankAccounts(ctx context.Context, issuerID int64) ([]domain.BankAccount, error) {
	query := `SELECT id,COALESCE(issuer_profile_id,0),bank_name,account_holder,account_number,card_number,iban,created_at,updated_at FROM bank_accounts`
	args := []any{}
	if issuerID > 0 {
		query += " WHERE issuer_profile_id=?"
		args = append(args, issuerID)
	}
	query += " ORDER BY id DESC LIMIT 500"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("خواندن حساب‌های بانکی: %w", err)
	}
	defer rows.Close()
	out := make([]domain.BankAccount, 0)
	for rows.Next() {
		var v domain.BankAccount
		var c, u string
		if err := rows.Scan(&v.ID, &v.IssuerProfileID, &v.BankName, &v.AccountHolder, &v.AccountNumber, &v.CardNumber, &v.IBAN, &c, &u); err != nil {
			return nil, err
		}
		v.CreatedAt, err = time.Parse(time.RFC3339Nano, c)
		if err != nil {
			return nil, err
		}
		v.UpdatedAt, err = time.Parse(time.RFC3339Nano, u)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) SaveDocumentTemplate(ctx context.Context, t domain.DocumentTemplate) (int64, error) {
	if err := t.Validate(); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var letterhead any
	if t.LetterheadID > 0 {
		letterhead = t.LetterheadID
	}
	var id int64
	if t.ID == 0 {
		r, e := tx.ExecContext(ctx, `INSERT INTO document_templates(name,prefix,thank_you,show_seal,show_signature,page_size,orientation,font_family,font_size,primary_color,accent_color,letterhead_id,show_logo,show_tax,show_bank_accounts,created_at,updated_at,logo_data,seal_data,signature_data) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.Name, t.Prefix, t.ThankYou, boolInt(t.ShowSeal), boolInt(t.ShowSignature), t.PageSize, t.Orientation, t.FontFamily, t.FontSize, t.PrimaryColor, t.AccentColor, letterhead, boolInt(t.ShowLogo), boolInt(t.ShowTax), boolInt(t.ShowBankAccounts), now, now, t.LogoData, t.SealData, t.SignatureData)
		if e != nil {
			return 0, fmt.Errorf("ذخیره قالب: %w", e)
		}
		id, e = r.LastInsertId()
		if e != nil {
			return 0, e
		}
	} else {
		r, e := tx.ExecContext(ctx, `UPDATE document_templates SET name=?,prefix=?,thank_you=?,show_seal=?,show_signature=?,page_size=?,orientation=?,font_family=?,font_size=?,primary_color=?,accent_color=?,letterhead_id=?,show_logo=?,show_tax=?,show_bank_accounts=?,updated_at=?,logo_data=?,seal_data=?,signature_data=? WHERE id=?`, t.Name, t.Prefix, t.ThankYou, boolInt(t.ShowSeal), boolInt(t.ShowSignature), t.PageSize, t.Orientation, t.FontFamily, t.FontSize, t.PrimaryColor, t.AccentColor, letterhead, boolInt(t.ShowLogo), boolInt(t.ShowTax), boolInt(t.ShowBankAccounts), now, t.LogoData, t.SealData, t.SignatureData, t.ID)
		if e != nil {
			return 0, fmt.Errorf("ویرایش قالب: %w", e)
		}
		n, e := r.RowsAffected()
		if e != nil {
			return 0, e
		}
		if n == 0 {
			return 0, errors.New("قالب پیدا نشد")
		}
		id = t.ID
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM template_bank_accounts WHERE template_id=?", id); err != nil {
		return 0, err
	}
	for position, accountID := range t.BankAccountIDs {
		if _, err = tx.ExecContext(ctx, "INSERT INTO template_bank_accounts(template_id,bank_account_id,position) VALUES(?,?,?)", id, accountID, position); err != nil {
			return 0, fmt.Errorf("اتصال حساب بانکی به قالب: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("ثبت قالب و حساب‌ها: %w", err)
	}
	return id, nil
}

func (s *Store) ListDocumentTemplates(ctx context.Context) ([]domain.DocumentTemplate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,prefix,thank_you,show_seal,show_signature,page_size,orientation,font_family,font_size,primary_color,accent_color,COALESCE(letterhead_id,0),show_logo,show_tax,show_bank_accounts,created_at,updated_at,logo_data,seal_data,signature_data FROM document_templates ORDER BY id DESC LIMIT 500`)
	if err != nil {
		return nil, fmt.Errorf("خواندن قالب‌ها: %w", err)
	}
	type entry struct {
		t    domain.DocumentTemplate
		c, u string
	}
	entries := make([]entry, 0)
	for rows.Next() {
		var e entry
		var logo, tax, banks, seal, signature int
		if err := rows.Scan(&e.t.ID, &e.t.Name, &e.t.Prefix, &e.t.ThankYou, &seal, &signature, &e.t.PageSize, &e.t.Orientation, &e.t.FontFamily, &e.t.FontSize, &e.t.PrimaryColor, &e.t.AccentColor, &e.t.LetterheadID, &logo, &tax, &banks, &e.c, &e.u, &e.t.LogoData, &e.t.SealData, &e.t.SignatureData); err != nil {
			rows.Close()
			return nil, err
		}
		e.t.ShowLogo = logo == 1
		e.t.ShowTax = tax == 1
		e.t.ShowBankAccounts = banks == 1
		e.t.ShowSeal = seal == 1
		e.t.ShowSignature = signature == 1
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].t.CreatedAt, err = time.Parse(time.RFC3339Nano, entries[i].c)
		if err != nil {
			return nil, err
		}
		entries[i].t.UpdatedAt, err = time.Parse(time.RFC3339Nano, entries[i].u)
		if err != nil {
			return nil, err
		}
		accounts, err := s.db.QueryContext(ctx, "SELECT bank_account_id FROM template_bank_accounts WHERE template_id=? ORDER BY position", entries[i].t.ID)
		if err != nil {
			return nil, err
		}
		for accounts.Next() {
			var id int64
			if err := accounts.Scan(&id); err != nil {
				accounts.Close()
				return nil, err
			}
			entries[i].t.BankAccountIDs = append(entries[i].t.BankAccountIDs, id)
		}
		if err := accounts.Err(); err != nil {
			accounts.Close()
			return nil, err
		}
		if err := accounts.Close(); err != nil {
			return nil, err
		}
	}
	out := make([]domain.DocumentTemplate, len(entries))
	for i, e := range entries {
		out[i] = e.t
	}
	return out, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// BankAccountByIDs returns only the requested accounts, preserving template order.
func (s *Store) BankAccountByIDs(ctx context.Context, ids []int64) ([]domain.BankAccount, error) {
	out := make([]domain.BankAccount, 0, len(ids))
	for _, id := range ids {
		var a domain.BankAccount
		var issuer sql.NullInt64
		var c, u string
		err := s.db.QueryRowContext(ctx, `SELECT id,issuer_profile_id,bank_name,account_holder,account_number,card_number,iban,created_at,updated_at FROM bank_accounts WHERE id=?`, id).Scan(&a.ID, &issuer, &a.BankName, &a.AccountHolder, &a.AccountNumber, &a.CardNumber, &a.IBAN, &c, &u)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("حساب بانکی %d پیدا نشد", id)
		}
		if err != nil {
			return nil, err
		}
		if issuer.Valid {
			a.IssuerProfileID = issuer.Int64
		}
		a.CreatedAt, err = time.Parse(time.RFC3339Nano, c)
		if err != nil {
			return nil, err
		}
		a.UpdatedAt, err = time.Parse(time.RFC3339Nano, u)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
