package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func (s *Store) CreateInvoiceDraft(ctx context.Context, doc domain.InvoiceDocument) (int64, string, error) {
	if err := doc.Validate(); err != nil {
		return 0, "", err
	}
	totals, err := domain.CalculateFinancialsWithCharges(doc.Items, doc.InvoiceDiscount, doc.TaxRateBPS, doc.AdditionalCharges)
	if err != nil {
		return 0, "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("شروع تراکنش فاکتور: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	series := strings.TrimSpace(doc.Numbering.Series)
	var numberValue int64
	err = tx.QueryRowContext(ctx, "SELECT next_value FROM numbering_sequences WHERE series=?", series).Scan(&numberValue)
	if errors.Is(err, sql.ErrNoRows) {
		numberValue = doc.Numbering.StartAt
		if numberValue == int64(^uint64(0)>>1) {
			return 0, "", errors.New("سرریز شماره‌گذاری فاکتور")
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO numbering_sequences(series,next_value,updated_at) VALUES(?,?,?)", series, numberValue+1, now)
	} else if err == nil {
		if numberValue <= 0 {
			return 0, "", errors.New("وضعیت شماره‌گذاری نامعتبر است")
		}
		if numberValue == int64(^uint64(0)>>1) {
			return 0, "", errors.New("سرریز شماره‌گذاری فاکتور")
		}
		_, err = tx.ExecContext(ctx, "UPDATE numbering_sequences SET next_value=?,updated_at=? WHERE series=?", numberValue+1, now, series)
	}
	if err != nil {
		return 0, "", fmt.Errorf("رزرو شماره فاکتور: %w", err)
	}
	number := fmt.Sprintf("%s%0*d%s", doc.Numbering.Prefix, doc.Numbering.Width, numberValue, doc.Numbering.Suffix)
	dueDate := ""
	if !doc.DueDate.IsZero() {
		dueDate = doc.DueDate.Format("2006-01-02")
	}
	discountTotal := int64(totals.ItemDiscount + totals.InvoiceDiscount)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO invoices(number,kind,status,currency,customer_id,issuer_profile_id,customer_snapshot,issuer_snapshot,
		 issue_date,due_date,subtotal_minor,discount_minor,tax_minor,total_minor,note,created_at,updated_at,
		 item_discount_minor,invoice_discount_mode,invoice_discount_value,tax_rate_bps,numbering_series,bank_accounts_snapshot,document_type,additional_charges_json,additional_charge_minor)
		VALUES(?,?,'draft',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		number, doc.Kind, string(doc.Currency), nullableID(doc.CustomerID), nullableID(doc.IssuerProfileID),
		doc.CustomerSnapshot, doc.IssuerSnapshot, doc.IssueDate.Format("2006-01-02"), dueDate,
		int64(totals.Subtotal), discountTotal, int64(totals.Tax), int64(totals.Total), doc.Note, now, now,
		int64(totals.ItemDiscount), discountModeOrFixed(doc.InvoiceDiscount.Mode), int64(doc.InvoiceDiscount.Value),
		doc.TaxRateBPS, series, invoiceBankSnapshot(doc.BankAccountsSnapshot), invoiceDocumentType(doc.DocumentType, doc.Kind), marshalAdditionalCharges(doc.AdditionalCharges), int64(totals.AdditionalCharges))
	if err != nil {
		return 0, "", fmt.Errorf("ذخیره پیش‌نویس فاکتور: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	for position, item := range doc.Items {
		lineTotal, err := item.LineTotal()
		if err != nil {
			return 0, "", err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO invoice_items(invoice_id,position,description,quantity,unit,unit_price,discount_minor,line_total)
			VALUES(?,?,?,?,?,?,?,?)`, id, position, strings.TrimSpace(item.Description), item.Quantity,
			strings.TrimSpace(item.Unit), int64(item.UnitPrice), int64(item.Discount), int64(lineTotal))
		if err != nil {
			return 0, "", fmt.Errorf("ذخیره قلم %d: %w", position+1, err)
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('invoice',?,'created',?,'{}')", id, now)
	if err != nil {
		return 0, "", fmt.Errorf("ثبت رویداد فاکتور: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("ثبت تراکنش فاکتور: %w", err)
	}
	return id, number, nil
}

func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func discountModeOrFixed(mode domain.DiscountMode) string {
	if mode == "" {
		return string(domain.DiscountFixed)
	}
	return string(mode)
}

func (s *Store) IssueInvoice(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("شناسه فاکتور نامعتبر است")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var itemCount int
	if err := tx.QueryRowContext(ctx, "SELECT status,(SELECT COUNT(*) FROM invoice_items WHERE invoice_id=invoices.id) FROM invoices WHERE id=?", id).Scan(&status, &itemCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("فاکتور پیدا نشد")
		}
		return err
	}
	var kind string
	if err := tx.QueryRowContext(ctx, "SELECT kind FROM invoices WHERE id=?", id).Scan(&kind); err != nil {
		return err
	}
	if kind == "proforma" {
		return errors.New("پیش‌فاکتور باید با عملیات تبدیل به فاکتور نهایی شود")
	}
	if status != string(domain.InvoiceDraft) {
		return errors.New("فقط پیش‌نویس فاکتور قابل صدور است")
	}
	if itemCount == 0 {
		return errors.New("فاکتور بدون قلم قابل صدور نیست")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, "UPDATE invoices SET status='issued',updated_at=? WHERE id=? AND status='draft'", now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('invoice',?,'issued',?,'{}')", id, now); err != nil {
		return err
	}
	return tx.Commit()
}

func invoiceBankSnapshot(value string) string {
	if strings.TrimSpace(value) == "" {
		return "[]"
	}
	return value
}

func invoiceDocumentType(value, kind string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if kind == "proforma" {
		return "proforma"
	}
	return "sales"
}

func marshalAdditionalCharges(charges []domain.AdditionalCharge) string {
	data, err := json.Marshal(charges)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func (s *Store) ListInvoiceDocuments(ctx context.Context, query string) ([]domain.InvoiceListItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id,i.number,i.kind,i.document_type,i.status,i.currency,i.customer_snapshot,i.issue_date,i.total_minor+i.additional_charge_minor,
			CASE WHEN i.kind='invoice' AND i.status='issued'
				THEN i.total_minor+i.additional_charge_minor-COALESCE((SELECT SUM(pa.amount_minor) FROM payment_allocations pa WHERE pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)),0)
				ELSE 0 END
		FROM invoices i
		WHERE (?='' OR i.number LIKE '%'||?||'%' OR i.customer_snapshot LIKE '%'||?||'%')
		ORDER BY i.id DESC`, strings.TrimSpace(query), strings.TrimSpace(query), strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.InvoiceListItem
	for rows.Next() {
		var item domain.InvoiceListItem
		if err := rows.Scan(&item.ID, &item.Number, &item.Kind, &item.DocumentType, &item.Status, &item.Currency, &item.Customer, &item.IssueDate, &item.Total, &item.Balance); err != nil {
			return nil, err
		}
		var customerSnapshot struct {
			Name      string `json:"name"`
			LegalName string `json:"legal_name"`
		}
		if json.Unmarshal([]byte(item.Customer), &customerSnapshot) == nil {
			item.Customer = customerSnapshot.Name
			if strings.TrimSpace(customerSnapshot.LegalName) != "" {
				item.Customer = customerSnapshot.LegalName
			}
		}
		if strings.TrimSpace(item.Customer) == "" {
			item.Customer = "مشتری حذف‌شده یا بدون نام"
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetInvoiceDocument(ctx context.Context, id int64) (domain.InvoiceDocument, error) {
	var doc domain.InvoiceDocument
	var status, currency, kind, issueDate, dueDate, discountMode, bankSnapshot, chargesJSON string
	var discountValue, taxRate int64
	err := s.db.QueryRowContext(ctx, `SELECT id,number,kind,status,currency,COALESCE(customer_id,0),COALESCE(issuer_profile_id,0),
		customer_snapshot,issuer_snapshot,issue_date,due_date,note,invoice_discount_mode,invoice_discount_value,tax_rate_bps,numbering_series,bank_accounts_snapshot,document_type,additional_charges_json
		FROM invoices WHERE id=?`, id).Scan(&doc.ID, &doc.Number, &kind, &status, &currency, &doc.CustomerID, &doc.IssuerProfileID,
		&doc.CustomerSnapshot, &doc.IssuerSnapshot, &issueDate, &dueDate, &doc.Note, &discountMode, &discountValue, &taxRate, &doc.Numbering.Series, &bankSnapshot, &doc.DocumentType, &chargesJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return doc, errors.New("سند پیدا نشد")
		}
		return doc, err
	}
	doc.Kind, doc.Status, doc.Currency = kind, domain.InvoiceStatus(status), domain.Currency(currency)
	doc.InvoiceDiscount = domain.Discount{Mode: domain.DiscountMode(discountMode), Value: domain.Money(discountValue)}
	doc.TaxRateBPS, doc.BankAccountsSnapshot = taxRate, bankSnapshot
	if err := json.Unmarshal([]byte(chargesJSON), &doc.AdditionalCharges); err != nil {
		return doc, fmt.Errorf("هزینه‌های اضافهٔ سند نامعتبر است: %w", err)
	}
	doc.Numbering = domain.NumberingConfig{Series: doc.Numbering.Series, StartAt: 1, Width: 6}
	doc.IssueDate, err = time.Parse("2006-01-02", issueDate)
	if err != nil {
		return doc, err
	}
	if dueDate != "" {
		doc.DueDate, err = time.Parse("2006-01-02", dueDate)
		if err != nil {
			return doc, err
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT description,quantity,unit,unit_price,discount_minor FROM invoice_items WHERE invoice_id=? ORDER BY position`, id)
	if err != nil {
		return doc, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.InvoiceItem
		var price, discount int64
		if err := rows.Scan(&item.Description, &item.Quantity, &item.Unit, &price, &discount); err != nil {
			return doc, err
		}
		item.UnitPrice, item.Discount = domain.Money(price), domain.Money(discount)
		doc.Items = append(doc.Items, item)
	}
	return doc, rows.Err()
}

func (s *Store) ConvertProforma(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("شناسه پیش‌فاکتور نامعتبر است")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var kind, status string
	var itemCount int
	err = tx.QueryRowContext(ctx, "SELECT kind,status,(SELECT COUNT(*) FROM invoice_items WHERE invoice_id=invoices.id) FROM invoices WHERE id=?", id).Scan(&kind, &status, &itemCount)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("پیش‌فاکتور پیدا نشد")
	}
	if err != nil {
		return err
	}
	if kind != "proforma" || status != string(domain.InvoiceDraft) {
		return errors.New("فقط پیش‌فاکتور پیش‌نویس قابل تبدیل است")
	}
	if itemCount == 0 {
		return errors.New("پیش‌فاکتور بدون قلم قابل تبدیل نیست")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "UPDATE invoices SET kind='invoice',document_type=CASE WHEN document_type='proforma' THEN 'sales' ELSE document_type END,status='issued',updated_at=? WHERE id=? AND kind='proforma' AND status='draft'", now, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('invoice',?,'proforma_converted',?,'{}')", id, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateInvoiceDraft(ctx context.Context, doc domain.InvoiceDocument) error {
	if doc.ID <= 0 {
		return errors.New("شناسه سند برای ویرایش نامعتبر است")
	}
	if err := doc.Validate(); err != nil {
		return err
	}
	totals, err := domain.CalculateFinancialsWithCharges(doc.Items, doc.InvoiceDiscount, doc.TaxRateBPS, doc.AdditionalCharges)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var currentKind, currentStatus, currentNumber string
	if err := tx.QueryRowContext(ctx, "SELECT kind,status,number FROM invoices WHERE id=?", doc.ID).Scan(&currentKind, &currentStatus, &currentNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("سند برای ویرایش پیدا نشد")
		}
		return err
	}
	if currentStatus != string(domain.InvoiceDraft) && currentStatus != string(domain.InvoiceIssued) {
		return errors.New("فقط پیش‌نویس یا فاکتور تسویه‌نشده قابل ویرایش است")
	}
	if currentStatus == string(domain.InvoiceIssued) {
		var paid, previousTotal int64
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_minor),0) FROM payment_allocations pa WHERE invoice_id=? AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id)", doc.ID).Scan(&paid); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, "SELECT total_minor+additional_charge_minor FROM invoices WHERE id=?", doc.ID).Scan(&previousTotal); err != nil {
			return err
		}
		if paid >= previousTotal {
			return errors.New("ÙØ§Ú©ØªÙˆØ± ØªØ³ÙˆÛŒÙ‡â€ŒØ´Ø¯Ù‡ Ù‚Ø§Ø¨Ù„ ÙˆÛŒØ±Ø§ÛŒØ´ Ù†ÛŒØ³Øª")
		}
		if int64(totals.GrandTotal) < paid {
			return errors.New("مبلغ نهایی نمی‌تواند کمتر از پرداخت‌های ثبت‌شده باشد")
		}
	}
	if currentKind != doc.Kind {
		return errors.New("نوع سند هنگام ویرایش قابل تغییر نیست")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	dueDate := ""
	if !doc.DueDate.IsZero() {
		dueDate = doc.DueDate.Format("2006-01-02")
	}
	discountTotal := int64(totals.ItemDiscount + totals.InvoiceDiscount)
	_, err = tx.ExecContext(ctx, `UPDATE invoices SET currency=?,customer_id=?,issuer_profile_id=?,customer_snapshot=?,issuer_snapshot=?,issue_date=?,due_date=?,
		subtotal_minor=?,discount_minor=?,tax_minor=?,total_minor=?,note=?,updated_at=?,item_discount_minor=?,invoice_discount_mode=?,
		invoice_discount_value=?,tax_rate_bps=?,bank_accounts_snapshot=?,document_type=?,additional_charges_json=?,additional_charge_minor=? WHERE id=? AND status IN ('draft','issued')`,
		string(doc.Currency), nullableID(doc.CustomerID), nullableID(doc.IssuerProfileID), doc.CustomerSnapshot, doc.IssuerSnapshot,
		doc.IssueDate.Format("2006-01-02"), dueDate, int64(totals.Subtotal), discountTotal, int64(totals.Tax), int64(totals.Total),
		doc.Note, now, int64(totals.ItemDiscount), discountModeOrFixed(doc.InvoiceDiscount.Mode), int64(doc.InvoiceDiscount.Value),
		doc.TaxRateBPS, invoiceBankSnapshot(doc.BankAccountsSnapshot), invoiceDocumentType(doc.DocumentType, doc.Kind), marshalAdditionalCharges(doc.AdditionalCharges), int64(totals.AdditionalCharges), doc.ID)
	if err != nil {
		return fmt.Errorf("به‌روزرسانی سند: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM invoice_items WHERE invoice_id=?", doc.ID); err != nil {
		return err
	}
	for position, item := range doc.Items {
		lineTotal, err := item.LineTotal()
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO invoice_items(invoice_id,position,description,quantity,unit,unit_price,discount_minor,line_total) VALUES(?,?,?,?,?,?,?,?)`,
			doc.ID, position, strings.TrimSpace(item.Description), item.Quantity, strings.TrimSpace(item.Unit), int64(item.UnitPrice), int64(item.Discount), int64(lineTotal)); err != nil {
			return fmt.Errorf("ذخیره قلم %d: %w", position+1, err)
		}
	}
	action := "draft_updated"
	if currentStatus == string(domain.InvoiceIssued) {
		action = "unsettled_invoice_updated"
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('invoice',?,?,?,'{}')", doc.ID, action, now); err != nil {
		return err
	}
	_ = currentNumber // the issued/reserved document number is immutable during edits
	return tx.Commit()
}

// SuggestedInvoiceItems uses issued invoices as the source of reusable items.
func (s *Store) SuggestedInvoiceItems(ctx context.Context, query string, currency domain.Currency) ([]domain.InvoiceItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT i.description, i.unit, i.unit_price FROM invoice_items i JOIN invoices d ON d.id=i.invoice_id WHERE d.status='issued' AND d.currency=? AND instr(lower(i.description), lower(?)) > 0 ORDER BY d.updated_at DESC, d.id DESC, i.position DESC`, currency, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.InvoiceItem
	seen := map[string]bool{}
	for rows.Next() {
		var item domain.InvoiceItem
		if err := rows.Scan(&item.Description, &item.Unit, &item.UnitPrice); err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimSpace(item.Description))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
		if len(result) == 20 {
			break
		}
	}
	return result, rows.Err()
}
