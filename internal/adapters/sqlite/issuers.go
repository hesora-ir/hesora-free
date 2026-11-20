package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/hesora-ir/hesora-free/internal/banking"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func normalizeIssuerProfile(profile domain.IssuerProfile) domain.IssuerProfile {
	profile.Type = strings.TrimSpace(profile.Type)
	if profile.Type == "" {
		profile.Type = domain.IssuerTypeIndividual
	}
	profile.Name = strings.TrimSpace(profile.Name)
	profile.FirstName = strings.TrimSpace(profile.FirstName)
	profile.LastName = strings.TrimSpace(profile.LastName)
	profile.LegalName = strings.TrimSpace(profile.LegalName)
	profile.TaxID = strings.TrimSpace(profile.TaxID)
	profile.Address = strings.TrimSpace(profile.Address)
	if len(profile.Phones) == 0 && strings.TrimSpace(profile.Phone) != "" {
		profile.Phones = []string{strings.TrimSpace(profile.Phone)}
	} else if len(profile.Phones) > 0 && profile.FirstName == "" && profile.LastName == "" && strings.TrimSpace(profile.Phone) != "" && strings.TrimSpace(profile.Phone) != strings.TrimSpace(profile.Phones[0]) {
		profile.Phones[0] = strings.TrimSpace(profile.Phone)
	}
	phones := make([]string, 0, len(profile.Phones))
	seen := make(map[string]struct{}, len(profile.Phones))
	for _, phone := range profile.Phones {
		phone = strings.TrimSpace(phone)
		if phone == "" {
			continue
		}
		if _, exists := seen[phone]; exists {
			continue
		}
		seen[phone] = struct{}{}
		phones = append(phones, phone)
	}
	profile.Phones = phones
	if len(profile.Phones) > 0 {
		profile.Phone = profile.Phones[0]
	} else {
		profile.Phone = ""
	}
	if profile.Type == domain.IssuerTypeIndividual && (profile.FirstName != "" || profile.LastName != "") {
		profile.Name = strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	} else if profile.Type == domain.IssuerTypeCompany && profile.LegalName != "" {
		profile.Name = profile.LegalName
	}
	return profile
}

func (s *Store) SaveIssuerProfile(ctx context.Context, profile domain.IssuerProfile) (int64, error) {
	profile = normalizeIssuerProfile(profile)
	if err := profile.Validate(); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("شروع تراکنش پروفایل صادرکننده: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	if profile.ID == 0 {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO issuer_profiles(profile_type, first_name, last_name, name, legal_name, tax_id, phone, email, address, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profile.Type, profile.FirstName, profile.LastName, profile.Name, profile.LegalName,
			profile.TaxID, profile.Phone, profile.Email, profile.Address, now, now)
		if err != nil {
			return 0, fmt.Errorf("ذخیره پروفایل صادرکننده: %w", err)
		}
		id, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE issuer_profiles SET profile_type=?, first_name=?, last_name=?, name=?, legal_name=?, tax_id=?, phone=?, email=?, address=?, updated_at=?
			WHERE id=?`,
			profile.Type, profile.FirstName, profile.LastName, profile.Name, profile.LegalName,
			profile.TaxID, profile.Phone, profile.Email, profile.Address, now, profile.ID)
		if err != nil {
			return 0, fmt.Errorf("ویرایش پروفایل صادرکننده: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if changed == 0 {
			return 0, errors.New("پروفایل صادرکننده پیدا نشد")
		}
		id = profile.ID
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM issuer_profile_phones WHERE issuer_profile_id=?", id); err != nil {
		return 0, fmt.Errorf("به‌روزرسانی شماره‌های تماس: %w", err)
	}
	for i, phone := range profile.Phones {
		if _, err := tx.ExecContext(ctx, "INSERT INTO issuer_profile_phones(issuer_profile_id, phone, position) VALUES(?, ?, ?)", id, phone, i); err != nil {
			return 0, fmt.Errorf("ذخیره شماره تماس: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM issuer_profile_bank_details WHERE issuer_profile_id=?", id); err != nil {
		return 0, fmt.Errorf("به‌روزرسانی حساب‌های صادرکننده: %w", err)
	}
	for i, account := range profile.BankDetails {
		if result := banking.ValidateIBAN(account.IBAN); result.Valid {
			account.IBAN = result.Normalized
		}
		// Profiles are editable current data: changing the IBAN must replace a
		// stale bank label. Historical invoice snapshots keep their recorded name.
		if detected := banking.ResolveBankName("", account.IBAN, account.CardNumber); detected != "" {
			account.BankName = detected
		}
		if strings.TrimSpace(account.CardNumber) == "" && strings.TrimSpace(account.AccountNumber) == "" && strings.TrimSpace(account.IBAN) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issuer_profile_bank_details(issuer_profile_id, card_number, account_number, iban, position, bank_name)
			VALUES(?, ?, ?, ?, ?, ?)`, id, strings.TrimSpace(account.CardNumber), strings.TrimSpace(account.AccountNumber), strings.TrimSpace(account.IBAN), i, strings.TrimSpace(account.BankName)); err != nil {
			return 0, fmt.Errorf("ذخیره حساب صادرکننده: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ثبت پروفایل صادرکننده: %w", err)
	}
	return id, nil
}

func (s *Store) ListIssuerProfiles(ctx context.Context) ([]domain.IssuerProfile, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM issuer_profiles ORDER BY id DESC LIMIT 200")
	if err != nil {
		return nil, fmt.Errorf("خواندن فهرست پروفایل‌های صادرکننده: %w", err)
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	profiles := make([]domain.IssuerProfile, 0, len(ids))
	for _, id := range ids {
		profile, err := s.IssuerProfileByID(ctx, id)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s *Store) IssuerProfileByID(ctx context.Context, id int64) (domain.IssuerProfile, error) {
	var p domain.IssuerProfile
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, profile_type, name, first_name, last_name, legal_name, tax_id, phone, email, address, created_at, updated_at
		FROM issuer_profiles WHERE id=?`, id).Scan(
		&p.ID, &p.Type, &p.Name, &p.FirstName, &p.LastName, &p.LegalName, &p.TaxID, &p.Phone, &p.Email, &p.Address, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IssuerProfile{}, errors.New("پروفایل صادرکننده پیدا نشد")
	}
	if err != nil {
		return domain.IssuerProfile{}, fmt.Errorf("خواندن پروفایل صادرکننده: %w", err)
	}
	p.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.IssuerProfile{}, err
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return domain.IssuerProfile{}, err
	}
	phoneRows, err := s.db.QueryContext(ctx, "SELECT phone FROM issuer_profile_phones WHERE issuer_profile_id=? ORDER BY position, id", id)
	if err != nil {
		return domain.IssuerProfile{}, err
	}
	for phoneRows.Next() {
		var phone string
		if err := phoneRows.Scan(&phone); err != nil {
			phoneRows.Close()
			return domain.IssuerProfile{}, err
		}
		p.Phones = append(p.Phones, phone)
	}
	if err := phoneRows.Err(); err != nil {
		phoneRows.Close()
		return domain.IssuerProfile{}, err
	}
	if err := phoneRows.Close(); err != nil {
		return domain.IssuerProfile{}, err
	}
	if len(p.Phones) == 0 && strings.TrimSpace(p.Phone) != "" {
		p.Phones = []string{p.Phone}
	}
	accountRows, err := s.db.QueryContext(ctx, `SELECT id, card_number, account_number, iban, bank_name
		FROM issuer_profile_bank_details WHERE issuer_profile_id=? ORDER BY position, id`, id)
	if err != nil {
		return domain.IssuerProfile{}, err
	}
	for accountRows.Next() {
		var account domain.IssuerBankDetails
		if err := accountRows.Scan(&account.ID, &account.CardNumber, &account.AccountNumber, &account.IBAN, &account.BankName); err != nil {
			accountRows.Close()
			return domain.IssuerProfile{}, err
		}
		account.BankName = banking.ResolveBankName(account.BankName, account.IBAN, account.CardNumber)
		p.BankDetails = append(p.BankDetails, account)
	}
	if err := accountRows.Err(); err != nil {
		accountRows.Close()
		return domain.IssuerProfile{}, err
	}
	if err := accountRows.Close(); err != nil {
		return domain.IssuerProfile{}, err
	}
	return p, nil
}
