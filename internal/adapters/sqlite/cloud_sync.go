package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

// ApplyCloudDocuments materializes web workspace documents in the desktop's
// regular SQLite tables so existing screens and reports can use them.
func (s *Store) ApplyCloudDocuments(ctx context.Context, docs []CloudDocument) (int, error) {
	return s.applyCloudDocuments(ctx, docs, nil)
}

// ApplyCloudDocumentsAtLocalCursor fails instead of overwriting desktop edits
// made after the corresponding push/snapshot boundary.
func (s *Store) ApplyCloudDocumentsAtLocalCursor(ctx context.Context, docs []CloudDocument, expectedLocalCursor int64) (int, error) {
	return s.applyCloudDocuments(ctx, docs, &expectedLocalCursor)
}

func (s *Store) applyCloudDocuments(ctx context.Context, docs []CloudDocument, expectedLocalCursor *int64) (int, error) {
	if len(docs) > 500 {
		return 0, errors.New("ØªØ¹Ø¯Ø§Ø¯ Ø§Ø³Ù†Ø§Ø¯ Ø§Ø¨Ø±ÛŒ Ø§Ø² Ø³Ù‚Ù Ù‡Ù…Ú¯Ø§Ù…â€ŒØ³Ø§Ø²ÛŒ Ø¨ÛŒØ´ØªØ± Ø§Ø³Øª")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE cloud_sync_control SET applying_remote=1 WHERE id=1"); err != nil {
		return 0, err
	}
	if expectedLocalCursor != nil {
		var actual int64
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM local_cloud_changes").Scan(&actual); err != nil {
			return 0, err
		}
		if actual != *expectedLocalCursor {
			return 0, errors.New("local data changed during cloud sync; retry sync before applying web changes")
		}
	}
	changed := 0
	for _, d := range docs {
		if d.Deleted {
			if err = applyCloudDeletion(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
			continue
		}
		if len(d.Data) == 0 || len(d.Data) > 65536 {
			return 0, fmt.Errorf("Ø³Ù†Ø¯ Ø§Ø¨Ø±ÛŒ %s Ø§Ù†Ø¯Ø§Ø²Ù‡Ù” Ù†Ø§Ù…Ø¹ØªØ¨Ø± Ø¯Ø§Ø±Ø¯", d.ID)
		}
		switch d.Kind {
		case "customer":
			if err = applyCloudCustomer(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
		case "issuer":
			if err = applyCloudIssuer(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
		case "invoice":
			if err = applyCloudInvoice(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
		case "template":
			if err = applyCloudTemplate(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
		case "expense":
			if err = applyCloudExpense(ctx, tx, d); err != nil {
				return 0, err
			}
			changed++
		case "product": // Desktop currently has no catalog inventory equivalent for web-only product rows.
		default:
			continue
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE cloud_sync_control SET applying_remote=0 WHERE id=1"); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return changed, nil
}

func applyCloudDeletion(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil || !found {
		return err
	}
	var table string
	switch d.Kind {
	case "customer":
		table = "customers"
	case "issuer":
		table = "issuer_profiles"
	case "invoice":
		table = "invoices"
	case "template":
		table = "document_templates"
	case "expense":
		table = "expenses"
	default:
		return nil // Web-only catalogs have no local row to remove.
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE id=?", id); err != nil {
		return fmt.Errorf("cannot apply cloud deletion for %s: local records still depend on it: %w", d.ID, err)
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM cloud_document_map WHERE kind=? AND cloud_id=?", d.Kind, d.ID)
	return err
}

type CloudDocument struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	Version      int64           `json:"version"`
	UpdatedAt    int64           `json:"updated_at"`
	ChangeCursor int64           `json:"change_cursor,omitempty"`
	Deleted      bool            `json:"deleted"`
	Data         json.RawMessage `json:"data"`
	BaseData     json.RawMessage `json:"-"`
	LocalID      int64           `json:"-"`
	Operation    string          `json:"-"`
}

// SeedDesktopCloudMappings mirrors the server's deterministic IDs used when it
// imports the first desktop backup, avoiding heuristic row matching on bootstrap.
func (s *Store) SeedDesktopCloudMappings(ctx context.Context, userID string) error {
	if decoded, err := hex.DecodeString(userID); err != nil || len(decoded) != 16 {
		return errors.New("invalid cloud account ID")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range []struct{ kind, table string }{{"customer", "customers"}, {"issuer", "issuer_profiles"}, {"invoice", "invoices"}, {"template", "document_templates"}, {"expense", "expenses"}} {
		rows, qerr := tx.QueryContext(ctx, "SELECT id FROM "+item.table+" ORDER BY id")
		if qerr != nil {
			return qerr
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if qerr = rows.Scan(&id); qerr != nil {
				rows.Close()
				return qerr
			}
			ids = append(ids, id)
		}
		if qerr = rows.Err(); qerr != nil {
			rows.Close()
			return qerr
		}
		rows.Close()
		for _, localID := range ids {
			sum := sha256.Sum256([]byte(fmt.Sprintf("hesora-desktop:%s:%s:%d", userID, item.kind, localID)))
			cloudID := hex.EncodeToString(sum[:16])
			if _, err = tx.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,remote_version,updated_at) VALUES(?,?,?,0,?) ON CONFLICT(kind,local_id) DO UPDATE SET cloud_id=excluded.cloud_id,remote_version=0,base_data='',updated_at=excluded.updated_at", item.kind, cloudID, localID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return err
			}
			if item.kind == "template" {
				if _, err = tx.ExecContext(ctx, "INSERT INTO local_cloud_changes(kind,local_id,operation,created_at) VALUES('template',?,'upsert',CURRENT_TIMESTAMP)", localID); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// PendingLocalCloudChanges returns a bounded, coalesced batch of local changes.
// New cloud IDs are generated once and persisted before any network request.
func (s *Store) PendingLocalCloudChanges(ctx context.Context, after int64, limit int) ([]CloudDocument, int64, bool, error) {
	if after < 0 {
		return nil, after, false, errors.New("invalid local cloud cursor")
	}
	limit = max(1, min(500, limit))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, after, false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT c.kind,c.local_id,c.operation,c.id
		FROM local_cloud_changes c JOIN (SELECT MAX(id) AS id FROM local_cloud_changes WHERE id>? GROUP BY kind,local_id) latest ON latest.id=c.id
		ORDER BY c.id LIMIT ?`, after, limit)
	if err != nil {
		return nil, after, false, err
	}
	type change struct {
		kind, operation string
		localID, cursor int64
	}
	changes := make([]change, 0)
	for rows.Next() {
		var c change
		if err = rows.Scan(&c.kind, &c.localID, &c.operation, &c.cursor); err != nil {
			rows.Close()
			return nil, after, false, err
		}
		changes = append(changes, c)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, after, false, err
	}
	rows.Close()
	if len(changes) == 0 {
		return []CloudDocument{}, after, false, tx.Commit()
	}
	cursor := changes[len(changes)-1].cursor
	docs := make([]CloudDocument, 0, len(changes))
	for _, c := range changes {
		var cloudID string
		var remoteVersion int64
		var baseData string
		mapErr := tx.QueryRowContext(ctx, "SELECT cloud_id,remote_version,base_data FROM cloud_document_map WHERE kind=? AND local_id=?", c.kind, c.localID).Scan(&cloudID, &remoteVersion, &baseData)
		if errors.Is(mapErr, sql.ErrNoRows) {
			if c.operation == "delete" {
				continue
			}
			id := make([]byte, 16)
			if _, err = rand.Read(id); err != nil {
				return nil, after, false, err
			}
			cloudID = hex.EncodeToString(id)
			_, err = tx.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,remote_version,updated_at) VALUES(?,?,?,0,?)", c.kind, cloudID, c.localID, time.Now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				return nil, after, false, err
			}
		} else if mapErr != nil {
			return nil, after, false, mapErr
		}
		doc := CloudDocument{ID: cloudID, Kind: c.kind, Version: remoteVersion, Deleted: c.operation == "delete", LocalID: c.localID, Operation: c.operation, ChangeCursor: c.cursor, BaseData: json.RawMessage(baseData)}
		if doc.Deleted {
			doc.Data = json.RawMessage(`{}`)
		} else if doc.Data, err = localCloudDocumentData(ctx, tx, c.kind, c.localID); err != nil {
			return nil, after, false, err
		}
		docs = append(docs, doc)
	}
	more := false
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM local_cloud_changes WHERE id>?)", cursor).Scan(&more); err != nil {
		return nil, after, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, after, false, err
	}
	return docs, cursor, more, nil
}

func (s *Store) AcknowledgeLocalCloudDocument(ctx context.Context, d CloudDocument, remoteVersion int64) error {
	if d.Operation == "delete" {
		_, err := s.db.ExecContext(ctx, "DELETE FROM cloud_document_map WHERE kind=? AND cloud_id=?", d.Kind, d.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE cloud_document_map SET remote_version=?,base_data=?,updated_at=? WHERE kind=? AND cloud_id=? AND local_id=?", remoteVersion, string(d.Data), time.Now().UTC().Format(time.RFC3339Nano), d.Kind, d.ID, d.LocalID)
	return err
}

func (s *Store) LocalCloudPushCursor(ctx context.Context) (int64, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM cloud_sync_state WHERE name='local_push_cursor'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var cursor int64
	if _, err = fmt.Sscan(value, &cursor); err != nil || cursor < 0 {
		return 0, false, errors.New("invalid local push cursor")
	}
	return cursor, true, nil
}

func (s *Store) SaveLocalCloudPushCursor(ctx context.Context, cursor int64) error {
	if cursor < 0 {
		return errors.New("invalid local push cursor")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO cloud_sync_state(name,value) VALUES('local_push_cursor',?)
		ON CONFLICT(name) DO UPDATE SET value=excluded.value
		WHERE CAST(excluded.value AS INTEGER)>=CAST(cloud_sync_state.value AS INTEGER)`, fmt.Sprint(cursor))
	return err
}

func (s *Store) LatestLocalCloudChangeID(ctx context.Context) (int64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM local_cloud_changes").Scan(&cursor)
	return cursor, err
}

func localCloudDocumentData(ctx context.Context, tx *sql.Tx, kind string, id int64) (json.RawMessage, error) {
	switch kind {
	case "customer":
		var c cloudCustomer
		err := tx.QueryRowContext(ctx, "SELECT id,customer_type,name,first_name,last_name,legal_name,tax_id,phone,email,address FROM customers WHERE id=?", id).
			Scan(&c.ID, &c.Type, &c.Name, &c.First, &c.Last, &c.Legal, &c.TaxID, &c.Phone, &c.Email, &c.Address)
		if err != nil {
			return nil, err
		}
		c.Phones, err = localCloudStrings(ctx, tx, "SELECT phone FROM customer_phones WHERE customer_id=? ORDER BY position,id", id)
		if err != nil {
			return nil, err
		}
		c.Phones = includeCloudPrimary(c.Phone, c.Phones)
		c.Addresses, err = localCloudStrings(ctx, tx, "SELECT address FROM customer_addresses WHERE customer_id=? ORDER BY position,id", id)
		if err != nil {
			return nil, err
		}
		c.Addresses = includeCloudPrimary(c.Address, c.Addresses)
		return json.Marshal(c)
	case "issuer":
		var c cloudIssuer
		err := tx.QueryRowContext(ctx, "SELECT id,profile_type,name,first_name,last_name,legal_name,tax_id,phone,email,address FROM issuer_profiles WHERE id=?", id).
			Scan(&c.ID, &c.Type, &c.Name, &c.First, &c.Last, &c.Legal, &c.TaxID, &c.Phone, &c.Email, &c.Address)
		if err != nil {
			return nil, err
		}
		c.Phones, err = localCloudStrings(ctx, tx, "SELECT phone FROM issuer_profile_phones WHERE issuer_profile_id=? ORDER BY position,id", id)
		if err != nil {
			return nil, err
		}
		c.Phones = includeCloudPrimary(c.Phone, c.Phones)
		rows, err := tx.QueryContext(ctx, "SELECT bank_name,card_number,account_number,iban FROM issuer_profile_bank_details WHERE issuer_profile_id=? ORDER BY position,id", id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var b cloudBank
			if err = rows.Scan(&b.BankName, &b.Card, &b.Account, &b.IBAN); err != nil {
				rows.Close()
				return nil, err
			}
			c.Banks = append(c.Banks, b)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		return json.Marshal(c)
	case "invoice":
		var c cloudInvoice
		var customerID, issuerID sql.NullInt64
		var issue, due, chargesJSON string
		err := tx.QueryRowContext(ctx, `SELECT id,number,kind,document_type,status,currency,customer_id,issuer_profile_id,customer_snapshot,issuer_snapshot,issue_date,due_date,note,bank_accounts_snapshot,additional_charges_json,invoice_discount_mode,invoice_discount_value,tax_rate_bps,numbering_series FROM invoices WHERE id=?`, id).
			Scan(&c.ID, &c.Number, &c.Kind, &c.DocumentType, &c.Status, &c.Currency, &customerID, &issuerID, &c.CustomerSnapshot, &c.IssuerSnapshot, &issue, &due, &c.Note, &c.BankSnapshot, &chargesJSON, &c.Discount.Mode, &c.Discount.Value, &c.TaxRate, &c.Numbering.Series)
		if err != nil {
			return nil, err
		}
		if customerID.Valid {
			c.CustomerID = customerID.Int64
		}
		if issuerID.Valid {
			c.IssuerID = issuerID.Int64
		}
		if c.Status != "issued" {
			c.Status = "draft"
		}
		c.IssueDate = datePart(issue) + "T00:00:00Z"
		if due != "" {
			c.DueDate = datePart(due) + "T00:00:00Z"
		}
		if c.Discount.Mode == "" {
			c.Discount.Mode = "fixed"
		}
		if c.DocumentType == "" {
			c.DocumentType = "sales"
		}
		if c.Numbering.Series == "" {
			c.Numbering.Series = "default"
		}
		if chargesJSON != "" {
			if err = json.Unmarshal([]byte(chargesJSON), &c.Charges); err != nil {
				return nil, err
			}
		}
		rows, err := tx.QueryContext(ctx, "SELECT description,quantity,unit,unit_price,discount_minor FROM invoice_items WHERE invoice_id=? ORDER BY position,id", id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var item cloudItem
			if err = rows.Scan(&item.Description, &item.Quantity, &item.Unit, &item.Price, &item.Discount); err != nil {
				rows.Close()
				return nil, err
			}
			c.Items = append(c.Items, item)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		if c.Items == nil {
			c.Items = []cloudItem{}
		}
		if c.Charges == nil {
			c.Charges = []cloudCharge{}
		}
		return json.Marshal(c)
	case "template":
		var c cloudTemplate
		err := tx.QueryRowContext(ctx, `SELECT id,name,prefix,thank_you,page_size,orientation,font_family,font_size,primary_color,accent_color,show_logo,show_tax,show_bank_accounts,show_seal,show_signature,logo_data,seal_data,signature_data FROM document_templates WHERE id=?`, id).
			Scan(&c.ID, &c.Name, &c.Prefix, &c.ThankYou, &c.PageSize, &c.Orientation, &c.FontFamily, &c.FontSize, &c.PrimaryColor, &c.AccentColor, &c.ShowLogo, &c.ShowTax, &c.ShowBankAccounts, &c.ShowSeal, &c.ShowSignature, &c.LogoData, &c.SealData, &c.SignatureData)
		if err != nil {
			return nil, err
		}
		c.ShowBank = c.ShowBankAccounts
		return json.Marshal(c)
	case "expense":
		var c cloudExpense
		err := tx.QueryRowContext(ctx, `SELECT id,amount_minor,currency,expense_date,category,note,CAST(strftime('%s',created_at) AS INTEGER) FROM expenses WHERE id=?`, id).Scan(&c.ID, &c.Amount, &c.Currency, &c.Date, &c.Category, &c.Note, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		return json.Marshal(c)
	default:
		return nil, fmt.Errorf("unsupported local cloud document kind %q", kind)
	}
}

func includeCloudPrimary(primary string, values []string) []string {
	if strings.TrimSpace(primary) == "" {
		return values
	}
	for _, value := range values {
		if value == primary {
			return values
		}
	}
	return append([]string{primary}, values...)
}

func localCloudStrings(ctx context.Context, tx *sql.Tx, query string, id int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) CloudDocumentCursor(ctx context.Context) (int64, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM cloud_sync_state WHERE name='document_cursor'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var cursor int64
	_, err = fmt.Sscan(value, &cursor)
	if err != nil || cursor < 0 {
		return 0, errors.New("invalid local cloud document cursor")
	}
	return cursor, nil
}

func (s *Store) SaveCloudDocumentCursor(ctx context.Context, cursor int64) error {
	if cursor < 0 {
		return errors.New("invalid cloud document cursor")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO cloud_sync_state(name,value) VALUES('document_cursor',?)
		ON CONFLICT(name) DO UPDATE SET value=excluded.value
		WHERE CAST(excluded.value AS INTEGER)>=CAST(cloud_sync_state.value AS INTEGER)`, fmt.Sprint(cursor))
	return err
}

type SyncedPaymentAllocation struct {
	DocumentID string `json:"document_id"`
	Number     string `json:"number,omitempty"`
	Amount     int64  `json:"amount"`
}

type SyncedPayment struct {
	ID             string                    `json:"id"`
	ExternalRef    string                    `json:"external_ref"`
	Amount         int64                     `json:"amount"`
	Currency       string                    `json:"currency"`
	PaidAt         int64                     `json:"paid_at"`
	Method         string                    `json:"method"`
	Note           string                    `json:"note"`
	ReversedAt     *int64                    `json:"reversed_at,omitempty"`
	ReversalReason string                    `json:"reversal_reason,omitempty"`
	Allocations    []SyncedPaymentAllocation `json:"allocations"`
}

// ExportPayments assigns a stable cloud ID to each local payment and resolves
// every local allocation through the invoice map established by document sync.
func (s *Store) ExportPayments(ctx context.Context) ([]SyncedPayment, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT p.id,p.external_ref,p.amount_minor,p.currency,p.paid_at,p.method,p.note FROM payments p WHERE NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=p.id) ORDER BY p.id")
	if err != nil {
		return nil, err
	}
	type localPayment struct {
		id                           int64
		ref                          string
		amount                       int64
		currency, paid, method, note string
	}
	local := []localPayment{}
	for rows.Next() {
		var p localPayment
		if err = rows.Scan(&p.id, &p.ref, &p.amount, &p.currency, &p.paid, &p.method, &p.note); err != nil {
			rows.Close()
			return nil, err
		}
		local = append(local, p)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]SyncedPayment, 0, len(local))
	for _, p := range local {
		var cloudID string
		err = s.db.QueryRowContext(ctx, "SELECT cloud_id FROM cloud_document_map WHERE kind='payment' AND local_id=?", p.id).Scan(&cloudID)
		if errors.Is(err, sql.ErrNoRows) {
			idBytes := make([]byte, 16)
			if _, err = rand.Read(idBytes); err != nil {
				return nil, err
			}
			cloudID = hex.EncodeToString(idBytes)
			_, err = s.db.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,updated_at) VALUES('payment',?,?,?)", cloudID, p.id, time.Now().UTC().Format(time.RFC3339Nano))
		}
		if err != nil {
			return nil, err
		}
		paidAt, parseErr := time.Parse(time.RFC3339Nano, p.paid)
		if parseErr != nil {
			paidAt, parseErr = time.Parse("2006-01-02", p.paid)
		}
		if parseErr != nil {
			return nil, fmt.Errorf("parse payment %d date: %w", p.id, parseErr)
		}
		payment := SyncedPayment{ID: cloudID, ExternalRef: p.ref, Amount: p.amount, Currency: p.currency, PaidAt: paidAt.UTC().Unix(), Method: p.method, Note: p.note, Allocations: []SyncedPaymentAllocation{}}
		allocRows, qErr := s.db.QueryContext(ctx, "SELECT pa.amount_minor,m.cloud_id,i.number FROM payment_allocations pa JOIN invoices i ON i.id=pa.invoice_id LEFT JOIN cloud_document_map m ON m.kind='invoice' AND m.local_id=i.id WHERE pa.payment_id=? ORDER BY pa.invoice_id", p.id)
		if qErr != nil {
			return nil, qErr
		}
		for allocRows.Next() {
			var a SyncedPaymentAllocation
			if qErr = allocRows.Scan(&a.Amount, &a.DocumentID, &a.Number); qErr != nil {
				allocRows.Close()
				return nil, qErr
			}
			if a.DocumentID == "" {
				allocRows.Close()
				return nil, fmt.Errorf("invoice %s must sync before its payment", a.Number)
			}
			payment.Allocations = append(payment.Allocations, a)
		}
		if qErr = allocRows.Err(); qErr != nil {
			allocRows.Close()
			return nil, qErr
		}
		allocRows.Close()
		out = append(out, payment)
	}
	return out, nil
}

// ApplyPayments imports web receipts and reversals into native SQLite records.
// Reversed receipt/allocation rows stay intact; a marker excludes them from
// balances while preserving them for history and audit.
func (s *Store) ApplyPayments(ctx context.Context, payments []SyncedPayment) (int, error) {
	if len(payments) > 1000 {
		return 0, errors.New("too many workspace payments")
	}
	changed := 0
	for _, p := range payments {
		rawID, decodeErr := hex.DecodeString(p.ID)
		if decodeErr != nil || len(rawID) != 16 || p.Amount <= 0 || (p.Currency != "IRT" && p.Currency != "IRR") || p.PaidAt <= 0 || len(p.Allocations) == 0 {
			return 0, fmt.Errorf("invalid synced payment %s", p.ID)
		}
		if err := (domain.Payment{Amount: domain.Money(p.Amount), Currency: domain.Currency(p.Currency), PaidAt: time.Unix(p.PaidAt, 0), ExternalRef: p.ExternalRef, Method: p.Method, Note: p.Note}).Validate(); err != nil {
			return 0, err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		var localID int64
		var found bool
		err = tx.QueryRowContext(ctx, "SELECT local_id FROM cloud_document_map WHERE kind='payment' AND cloud_id=?", p.ID).Scan(&localID)
		if err == nil {
			found = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return 0, err
		}
		if found {
			var exists int
			if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE id=?", localID).Scan(&exists); err != nil {
				tx.Rollback()
				return 0, err
			}
			if exists == 0 {
				tx.Rollback()
				continue
			}
			if p.ReversedAt != nil {
				var alreadyReversed int
				if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cloud_payment_reversals WHERE cloud_id=?", p.ID).Scan(&alreadyReversed); err != nil {
					tx.Rollback()
					return 0, err
				}
				if alreadyReversed == 0 {
					if err = archivePaymentReversal(ctx, tx, p.ID, localID, *p.ReversedAt, p.ReversalReason); err != nil {
						tx.Rollback()
						return 0, err
					}
					changed++
				}
			}
			if err = tx.Commit(); err != nil {
				return 0, err
			}
			continue
		}
		var allocationsTotal int64
		seenInvoices := make(map[string]bool, len(p.Allocations))
		localAllocations := make([]struct{ invoiceID, amount int64 }, 0, len(p.Allocations))
		for _, a := range p.Allocations {
			if a.Amount <= 0 || seenInvoices[a.DocumentID] || a.Amount > p.Amount-allocationsTotal {
				tx.Rollback()
				return 0, errors.New("invalid payment allocations")
			}
			seenInvoices[a.DocumentID] = true
			var invoiceID int64
			if err = tx.QueryRowContext(ctx, "SELECT local_id FROM cloud_document_map WHERE kind='invoice' AND cloud_id=?", a.DocumentID).Scan(&invoiceID); err != nil {
				tx.Rollback()
				return 0, fmt.Errorf("payment invoice %s has not synced: %w", a.Number, err)
			}
			var status, currency string
			var total, allocated int64
			if err = tx.QueryRowContext(ctx, "SELECT status,currency,total_minor+additional_charge_minor FROM invoices WHERE id=?", invoiceID).Scan(&status, &currency, &total); err != nil {
				tx.Rollback()
				return 0, err
			}
			if status != "issued" || currency != p.Currency {
				tx.Rollback()
				return 0, fmt.Errorf("payment allocation is incompatible with invoice %s", a.Number)
			}
			if err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(pa.amount_minor),0) FROM payment_allocations pa LEFT JOIN cloud_payment_reversals r ON r.local_payment_id=pa.payment_id WHERE pa.invoice_id=? AND r.local_payment_id IS NULL", invoiceID).Scan(&allocated); err != nil {
				tx.Rollback()
				return 0, err
			}
			if a.Amount <= 0 || (!pReversed(p) && (allocated > total || a.Amount > total-allocated)) {
				tx.Rollback()
				return 0, fmt.Errorf("payment exceeds balance for invoice %s", a.Number)
			}
			allocationsTotal += a.Amount
			localAllocations = append(localAllocations, struct{ invoiceID, amount int64 }{invoiceID, a.Amount})
		}
		if allocationsTotal != p.Amount {
			tx.Rollback()
			return 0, errors.New("payment allocations must equal payment amount")
		}
		paidAt := time.Unix(p.PaidAt, 0).UTC().Format(time.RFC3339Nano)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		res, insertErr := tx.ExecContext(ctx, "INSERT INTO payments(external_ref,amount_minor,currency,paid_at,method,note,created_at) VALUES(?,?,?,?,?,?,?)", p.ExternalRef, p.Amount, p.Currency, paidAt, p.Method, p.Note, now)
		if insertErr != nil {
			tx.Rollback()
			return 0, insertErr
		}
		localID, err = res.LastInsertId()
		if err != nil {
			tx.Rollback()
			return 0, err
		}
		for _, a := range localAllocations {
			if _, err = tx.ExecContext(ctx, "INSERT INTO payment_allocations(payment_id,invoice_id,amount_minor,created_at) VALUES(?,?,?,?)", localID, a.invoiceID, a.amount, now); err != nil {
				tx.Rollback()
				return 0, err
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,updated_at) VALUES('payment',?,?,?)", p.ID, localID, now); err != nil {
			tx.Rollback()
			return 0, err
		}
		details, _ := json.Marshal(p)
		if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('payment',?,'cloud_imported',?,?)", localID, now, string(details)); err != nil {
			tx.Rollback()
			return 0, err
		}
		if p.ReversedAt != nil {
			if err = archivePaymentReversal(ctx, tx, p.ID, localID, *p.ReversedAt, p.ReversalReason); err != nil {
				tx.Rollback()
				return 0, err
			}
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

func pReversed(p SyncedPayment) bool { return p.ReversedAt != nil }

func archivePaymentReversal(ctx context.Context, tx *sql.Tx, cloudID string, id, when int64, reason string) error {
	var ref, currency, paidAt, method, note string
	var amount int64
	if err := tx.QueryRowContext(ctx, "SELECT external_ref,amount_minor,currency,paid_at,method,note FROM payments WHERE id=?", id).Scan(&ref, &amount, &currency, &paidAt, &method, &note); err != nil {
		return err
	}
	allocations, err := tx.QueryContext(ctx, "SELECT invoice_id,amount_minor FROM payment_allocations WHERE payment_id=? ORDER BY invoice_id", id)
	if err != nil {
		return err
	}
	type allocation struct {
		InvoiceID int64 `json:"invoice_id"`
		Amount    int64 `json:"amount"`
	}
	list := []allocation{}
	for allocations.Next() {
		var a allocation
		if err = allocations.Scan(&a.InvoiceID, &a.Amount); err != nil {
			allocations.Close()
			return err
		}
		list = append(list, a)
	}
	if err = allocations.Err(); err != nil {
		allocations.Close()
		return err
	}
	allocations.Close()
	details, _ := json.Marshal(map[string]any{"external_ref": ref, "amount": amount, "currency": currency, "paid_at": paidAt, "method": method, "note": note, "allocations": list, "reversed_at": when, "reason": reason})
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_events(entity_type,entity_id,action,occurred_at,details) VALUES('payment',?,'cloud_reversed',?,?)", id, now, string(details)); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO cloud_payment_reversals(cloud_id,local_payment_id,reversed_at,reason) VALUES(?,?,?,?) ON CONFLICT(cloud_id) DO NOTHING", cloudID, id, when, reason)
	return err
}

type cloudCustomer struct {
	ID        int64    `json:"ID"`
	Type      string   `json:"Type"`
	Name      string   `json:"Name"`
	First     string   `json:"FirstName"`
	Last      string   `json:"LastName"`
	Legal     string   `json:"LegalName"`
	TaxID     string   `json:"TaxID"`
	Phone     string   `json:"Phone"`
	Phones    []string `json:"Phones"`
	Email     string   `json:"Email"`
	Address   string   `json:"Address"`
	Addresses []string `json:"Addresses"`
}
type cloudBank struct {
	BankName string `json:"BankName"`
	Card     string `json:"CardNumber"`
	Account  string `json:"AccountNumber"`
	IBAN     string `json:"IBAN"`
}
type cloudIssuer struct {
	ID      int64       `json:"ID"`
	Type    string      `json:"Type"`
	Name    string      `json:"Name"`
	First   string      `json:"FirstName"`
	Last    string      `json:"LastName"`
	Legal   string      `json:"LegalName"`
	TaxID   string      `json:"TaxID"`
	Phone   string      `json:"Phone"`
	Phones  []string    `json:"Phones"`
	Email   string      `json:"Email"`
	Address string      `json:"Address"`
	Banks   []cloudBank `json:"BankDetails"`
}
type cloudItem struct {
	Description string `json:"Description"`
	Unit        string `json:"Unit"`
	Quantity    int64  `json:"Quantity"`
	Price       int64  `json:"UnitPrice"`
	Discount    int64  `json:"Discount"`
}
type cloudInvoiceDiscount struct {
	Mode  string `json:"Mode"`
	Value int64  `json:"Value"`
}
type cloudNumbering struct {
	Series string `json:"Series"`
}
type cloudCharge struct {
	Description string `json:"Description"`
	Amount      int64  `json:"Amount"`
}
type cloudInvoice struct {
	ID               int64                `json:"ID"`
	Number           string               `json:"Number"`
	Kind             string               `json:"Kind"`
	DocumentType     string               `json:"DocumentType"`
	Status           string               `json:"Status"`
	Currency         string               `json:"Currency"`
	CustomerID       int64                `json:"CustomerID"`
	IssuerID         int64                `json:"IssuerProfileID"`
	CustomerSnapshot string               `json:"CustomerSnapshot"`
	IssuerSnapshot   string               `json:"IssuerSnapshot"`
	IssueDate        string               `json:"IssueDate"`
	DueDate          string               `json:"DueDate"`
	Note             string               `json:"Note"`
	BankSnapshot     string               `json:"BankAccountsSnapshot"`
	Items            []cloudItem          `json:"Items"`
	Charges          []cloudCharge        `json:"AdditionalCharges"`
	Discount         cloudInvoiceDiscount `json:"InvoiceDiscount"`
	TaxRate          int64                `json:"TaxRateBPS"`
	Numbering        cloudNumbering       `json:"Numbering"`
}

type cloudTemplate struct {
	ID               int64  `json:"ID"`
	Name             string `json:"Name"`
	Prefix           string `json:"Prefix,omitempty"`
	PageSize         string `json:"PageSize,omitempty"`
	Orientation      string `json:"Orientation,omitempty"`
	FontFamily       string `json:"FontFamily,omitempty"`
	FontSize         int    `json:"FontSize,omitempty"`
	PrimaryColor     string `json:"PrimaryColor,omitempty"`
	AccentColor      string `json:"AccentColor,omitempty"`
	ShowLogo         *bool  `json:"ShowLogo,omitempty"`
	ShowTax          *bool  `json:"ShowTax,omitempty"`
	ShowBankAccounts *bool  `json:"ShowBankAccounts,omitempty"`
	ShowBank         *bool  `json:"ShowBank,omitempty"`
	ShowSeal         *bool  `json:"ShowSeal,omitempty"`
	ShowSignature    *bool  `json:"ShowSignature,omitempty"`
	ThankYou         string `json:"ThankYou,omitempty"`
	LogoData         string `json:"LogoData"`
	SealData         string `json:"SealData"`
	SignatureData    string `json:"SignatureData"`
}

type cloudExpense struct {
	ID        int64  `json:"ID"`
	Amount    int64  `json:"Amount"`
	Currency  string `json:"Currency"`
	Date      string `json:"Date"`
	Category  string `json:"Category"`
	Note      string `json:"Note"`
	CreatedAt int64  `json:"CreatedAt,omitempty"`
}

func syncMapID(ctx context.Context, tx *sql.Tx, kind, id string) (int64, bool, error) {
	var local int64
	err := tx.QueryRowContext(ctx, "SELECT local_id FROM cloud_document_map WHERE kind=? AND cloud_id=?", kind, id).Scan(&local)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return local, err == nil, err
}
func saveSyncMap(ctx context.Context, tx *sql.Tx, kind, id string, local int64, version int64, data json.RawMessage) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,remote_version,updated_at,base_data) VALUES(?,?,?,?,?,?) ON CONFLICT(kind,cloud_id) DO UPDATE SET local_id=excluded.local_id,remote_version=excluded.remote_version,updated_at=excluded.updated_at,base_data=excluded.base_data", kind, id, local, version, time.Now().UTC().Format(time.RFC3339Nano), string(data))
	return err
}

func applyCloudExpense(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	var c cloudExpense
	if err := json.Unmarshal(d.Data, &c); err != nil {
		return err
	}
	c.Category = strings.TrimSpace(c.Category)
	date, err := time.Parse("2006-01-02", c.Date)
	if err != nil || date.Format("2006-01-02") != c.Date || c.Amount <= 0 || (c.Currency != "IRT" && c.Currency != "IRR") || c.Category == "" || len(c.Category) > 120 || len(c.Note) > 2000 {
		return errors.New("اطلاعات هزینه ابری نامعتبر است")
	}
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	created := now
	if c.CreatedAt > 0 {
		created = time.Unix(c.CreatedAt, 0).UTC().Format(time.RFC3339Nano)
	}
	if found {
		_, err = tx.ExecContext(ctx, `UPDATE expenses SET amount_minor=?,currency=?,expense_date=?,category=?,note=? WHERE id=?`, c.Amount, c.Currency, c.Date, c.Category, c.Note, id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `INSERT INTO expenses(amount_minor,currency,expense_date,category,note,created_at) VALUES(?,?,?,?,?,?)`, c.Amount, c.Currency, c.Date, c.Category, c.Note, created)
		if err == nil {
			id, err = result.LastInsertId()
		}
	}
	if err != nil {
		return err
	}
	return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
}

func applyCloudTemplate(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	var c cloudTemplate
	if err := json.Unmarshal(d.Data, &c); err != nil {
		return err
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" || len(c.Name) > 200 {
		return errors.New("نام قالب چاپ ابری معتبر نیست")
	}
	if c.PageSize == "" {
		c.PageSize = "A4"
	}
	if c.Prefix == "" {
		c.Prefix = "FACT-"
	}
	if c.Orientation == "" {
		c.Orientation = "portrait"
	}
	if c.FontFamily == "" {
		c.FontFamily = "IRANSansX"
	}
	if c.FontSize == 0 {
		c.FontSize = 11
	}
	if c.PrimaryColor == "" {
		c.PrimaryColor = "#192631"
	}
	if c.AccentColor == "" {
		c.AccentColor = "#7058ED"
	}
	showLogo, showTax, showBanks := true, true, true
	if c.ShowLogo != nil {
		showLogo = *c.ShowLogo
	}
	if c.ShowTax != nil {
		showTax = *c.ShowTax
	}
	if c.ShowBankAccounts != nil {
		showBanks = *c.ShowBankAccounts
	}
	if c.ShowBank != nil {
		showBanks = *c.ShowBank
	}
	showSeal, showSignature := false, false
	if c.ShowSeal != nil {
		showSeal = *c.ShowSeal
	}
	if c.ShowSignature != nil {
		showSignature = *c.ShowSignature
	}
	t := domain.DocumentTemplate{Name: c.Name, Prefix: c.Prefix, ThankYou: c.ThankYou, LogoData: c.LogoData, SealData: c.SealData, SignatureData: c.SignatureData, PageSize: c.PageSize, Orientation: c.Orientation, FontFamily: c.FontFamily, FontSize: c.FontSize, PrimaryColor: c.PrimaryColor, AccentColor: c.AccentColor, ShowLogo: showLogo, ShowTax: showTax, ShowBankAccounts: showBanks, ShowSeal: showSeal, ShowSignature: showSignature}
	if err := t.Validate(); err != nil {
		return fmt.Errorf("قالب چاپ ابری: %w", err)
	}
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil {
		return err
	}
	if found {
		_, err = tx.ExecContext(ctx, `UPDATE document_templates SET name=?,prefix=?,thank_you=?,show_seal=?,show_signature=?,page_size=?,orientation=?,font_family=?,font_size=?,primary_color=?,accent_color=?,show_logo=?,show_tax=?,show_bank_accounts=?,logo_data=?,seal_data=?,signature_data=?,updated_at=? WHERE id=?`, t.Name, t.Prefix, t.ThankYou, boolInt(t.ShowSeal), boolInt(t.ShowSignature), t.PageSize, t.Orientation, t.FontFamily, t.FontSize, t.PrimaryColor, t.AccentColor, boolInt(t.ShowLogo), boolInt(t.ShowTax), boolInt(t.ShowBankAccounts), t.LogoData, t.SealData, t.SignatureData, time.Now().UTC().Format(time.RFC3339Nano), id)
		if err != nil {
			return err
		}
		return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
	}
	var duplicate int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM document_templates WHERE name=?", t.Name).Scan(&duplicate)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		shortID := d.ID
		if len(shortID) > 6 {
			shortID = shortID[:6]
		}
		suffix := " (وب " + shortID + ")"
		name := []rune(t.Name)
		for len(name) > 0 && len(string(name)+suffix) > 200 {
			name = name[:len(name)-1]
		}
		t.Name = strings.TrimSpace(string(name)) + suffix
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `INSERT INTO document_templates(name,prefix,thank_you,show_seal,show_signature,page_size,orientation,font_family,font_size,primary_color,accent_color,show_logo,show_tax,show_bank_accounts,created_at,updated_at,logo_data,seal_data,signature_data) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.Name, t.Prefix, t.ThankYou, boolInt(t.ShowSeal), boolInt(t.ShowSignature), t.PageSize, t.Orientation, t.FontFamily, t.FontSize, t.PrimaryColor, t.AccentColor, boolInt(t.ShowLogo), boolInt(t.ShowTax), boolInt(t.ShowBankAccounts), now, now, t.LogoData, t.SealData, t.SignatureData)
	if err != nil {
		return err
	}
	id, err = result.LastInsertId()
	if err != nil {
		return err
	}
	return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
}

func applyCloudCustomer(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	var c cloudCustomer
	if err := json.Unmarshal(d.Data, &c); err != nil {
		return err
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("Ù…Ø´ØªØ±ÛŒ Ø§Ø¨Ø±ÛŒ Ø¨Ø¯ÙˆÙ† Ù†Ø§Ù… Ø§Ø³Øª")
	}
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil {
		return err
	}
	if !found {
		err = tx.QueryRowContext(ctx, "SELECT id FROM customers WHERE name=? AND (phone=? OR email=?) ORDER BY id LIMIT 1", c.Name, c.Phone, c.Email).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found = err == nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if found {
		_, err = tx.ExecContext(ctx, "UPDATE customers SET customer_type=?,name=?,first_name=?,last_name=?,legal_name=?,tax_id=?,phone=?,email=?,address=?,updated_at=? WHERE id=?", valueOr(c.Type, "individual"), c.Name, c.First, c.Last, c.Legal, c.TaxID, c.Phone, c.Email, c.Address, now, id)
	} else {
		r, e := tx.ExecContext(ctx, "INSERT INTO customers(customer_type,name,first_name,last_name,legal_name,tax_id,phone,email,address,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", valueOr(c.Type, "individual"), c.Name, c.First, c.Last, c.Legal, c.TaxID, c.Phone, c.Email, c.Address, now, now)
		if e != nil {
			return e
		}
		id, err = r.LastInsertId()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM customer_phones WHERE customer_id=?", id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM customer_addresses WHERE customer_id=?", id); err != nil {
		return err
	}
	phones := c.Phones
	if len(phones) == 0 && c.Phone != "" {
		phones = []string{c.Phone}
	}
	for i, p := range phones {
		if strings.TrimSpace(p) != "" {
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO customer_phones(customer_id,phone,position) VALUES(?,?,?)", id, strings.TrimSpace(p), i); err != nil {
				return err
			}
		}
	}
	addresses := c.Addresses
	if len(addresses) == 0 && c.Address != "" {
		addresses = []string{c.Address}
	}
	for i, a := range addresses {
		if strings.TrimSpace(a) != "" {
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO customer_addresses(customer_id,address,position) VALUES(?,?,?)", id, strings.TrimSpace(a), i); err != nil {
				return err
			}
		}
	}
	return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
}
func applyCloudIssuer(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	var c cloudIssuer
	if err := json.Unmarshal(d.Data, &c); err != nil {
		return err
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("ÙØ±ÙˆØ´Ú¯Ø§Ù‡ Ø§Ø¨Ø±ÛŒ Ø¨Ø¯ÙˆÙ† Ù†Ø§Ù… Ø§Ø³Øª")
	}
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil {
		return err
	}
	if !found {
		err = tx.QueryRowContext(ctx, "SELECT id FROM issuer_profiles WHERE name=? ORDER BY id LIMIT 1", c.Name).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found = err == nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if found {
		_, err = tx.ExecContext(ctx, "UPDATE issuer_profiles SET profile_type=?,name=?,first_name=?,last_name=?,legal_name=?,tax_id=?,phone=?,email=?,address=?,updated_at=? WHERE id=?", valueOr(c.Type, "individual"), c.Name, c.First, c.Last, c.Legal, c.TaxID, c.Phone, c.Email, c.Address, now, id)
	} else {
		r, e := tx.ExecContext(ctx, "INSERT INTO issuer_profiles(profile_type,name,first_name,last_name,legal_name,tax_id,phone,email,address,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", valueOr(c.Type, "individual"), c.Name, c.First, c.Last, c.Legal, c.TaxID, c.Phone, c.Email, c.Address, now, now)
		if e != nil {
			return e
		}
		id, err = r.LastInsertId()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM issuer_profile_bank_details WHERE issuer_profile_id=?", id); err != nil {
		return err
	}
	for i, b := range c.Banks {
		if b.Card == "" && b.Account == "" && b.IBAN == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO issuer_profile_bank_details(issuer_profile_id,bank_name,card_number,account_number,iban,position) VALUES(?,?,?,?,?,?)", id, b.BankName, b.Card, b.Account, b.IBAN, i); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM issuer_profile_phones WHERE issuer_profile_id=?", id); err != nil {
		return err
	}
	phones := c.Phones
	if len(phones) == 0 && c.Phone != "" {
		phones = []string{c.Phone}
	}
	for i, p := range phones {
		if strings.TrimSpace(p) != "" {
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO issuer_profile_phones(issuer_profile_id,phone,position) VALUES(?,?,?)", id, strings.TrimSpace(p), i); err != nil {
				return err
			}
		}
	}
	return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
}
func valueOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
func applyCloudInvoice(ctx context.Context, tx *sql.Tx, d CloudDocument) error {
	var c cloudInvoice
	if err := json.Unmarshal(d.Data, &c); err != nil {
		return err
	}
	if strings.TrimSpace(c.Number) == "" {
		return errors.New("Ø´Ù…Ø§Ø±Ù‡ ÙØ§Ú©ØªÙˆØ± Ø§Ø¨Ø±ÛŒ Ø®Ø§Ù„ÛŒ Ø§Ø³Øª")
	}
	if c.Kind != "invoice" && c.Kind != "proforma" && c.Kind != "settlement" {
		return errors.New("Ù†ÙˆØ¹ ÙØ§Ú©ØªÙˆØ± Ø§Ø¨Ø±ÛŒ Ù†Ø§Ù…Ø¹ØªØ¨Ø± Ø§Ø³Øª")
	}
	if c.Status != "draft" && c.Status != "issued" && c.Status != "void" {
		return errors.New("ÙˆØ¶Ø¹ÛŒØª ÙØ§Ú©ØªÙˆØ± Ø§Ø¨Ø±ÛŒ Ù†Ø§Ù…Ø¹ØªØ¨Ø± Ø§Ø³Øª")
	}
	id, found, err := syncMapID(ctx, tx, d.Kind, d.ID)
	if err != nil {
		return err
	}
	if !found {
		err = tx.QueryRowContext(ctx, "SELECT id FROM invoices WHERE number=?", c.Number).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		found = err == nil
	}
	customerID := lookupLinkedCustomer(ctx, tx, c.CustomerID, c.CustomerSnapshot)
	issuerID := lookupLinkedIssuer(ctx, tx, c.IssuerID, c.IssuerSnapshot)
	if customerID == 0 || issuerID == 0 {
		return fmt.Errorf("Ø¨Ø±Ø§ÛŒ ÙØ§Ú©ØªÙˆØ± %s Ù…Ø´ØªØ±ÛŒ ÛŒØ§ ÙØ±ÙˆØ´Ú¯Ø§Ù‡ Ù…ØªÙ†Ø§Ø¸Ø± Ù¾ÛŒØ¯Ø§ Ù†Ø´Ø¯", c.Number)
	}
	currency := c.Currency
	if currency != "IRR" && currency != "IRT" {
		currency = "IRT"
	}
	issue := datePart(c.IssueDate)
	due := datePart(c.DueDate)
	if issue == "" {
		issue = time.Now().Format("2006-01-02")
	}
	status := c.Status
	now := time.Now().UTC().Format(time.RFC3339Nano)
	financialItems := make([]domain.InvoiceItem, 0, len(c.Items))
	for _, it := range c.Items {
		financialItems = append(financialItems, domain.InvoiceItem{Description: it.Description, Quantity: it.Quantity, Unit: it.Unit, UnitPrice: domain.Money(it.Price), Discount: domain.Money(it.Discount)})
	}
	financialCharges := make([]domain.AdditionalCharge, 0, len(c.Charges))
	for _, charge := range c.Charges {
		financialCharges = append(financialCharges, domain.AdditionalCharge{Description: charge.Description, Amount: domain.Money(charge.Amount)})
	}
	discountMode := domain.DiscountMode(valueOr(c.Discount.Mode, string(domain.DiscountFixed)))
	financialTotals, err := domain.CalculateFinancialsWithCharges(financialItems, domain.Discount{Mode: discountMode, Value: domain.Money(c.Discount.Value)}, c.TaxRate, financialCharges)
	if err != nil {
		return fmt.Errorf("calculate cloud invoice totals: %w", err)
	}
	sub := int64(financialTotals.Subtotal)
	itemDiscount := int64(financialTotals.ItemDiscount)
	discount := int64(financialTotals.InvoiceDiscount)
	tax := int64(financialTotals.Tax)
	chargeSum := int64(financialTotals.AdditionalCharges)
	chargeJSON := "[]"
	if len(c.Charges) > 0 {
		raw, marshalErr := json.Marshal(c.Charges)
		if marshalErr != nil {
			return marshalErr
		}
		chargeJSON = string(raw)
	}
	total := int64(financialTotals.Total)
	if c.CustomerSnapshot == "" {
		c.CustomerSnapshot = "{}"
	}
	if c.IssuerSnapshot == "" {
		c.IssuerSnapshot = "{}"
	}
	if c.BankSnapshot == "" {
		c.BankSnapshot = "[]"
	}
	series := valueOr(c.Numbering.Series, "default")
	if found {
		var paid, oldTotal int64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(pa.amount_minor),0),COALESCE(i.total_minor+i.additional_charge_minor,0) FROM invoices i LEFT JOIN payment_allocations pa ON pa.invoice_id=i.id AND NOT EXISTS(SELECT 1 FROM cloud_payment_reversals r WHERE r.local_payment_id=pa.payment_id) WHERE i.id=? GROUP BY i.id", id).Scan(&paid, &oldTotal)
		if paid >= oldTotal && paid > 0 && total != oldTotal {
			return fmt.Errorf("ÙØ§Ú©ØªÙˆØ± ØªØ³ÙˆÛŒÙ‡â€ŒØ´Ø¯Ù‡Ù” %s Ù‚Ø§Ø¨Ù„ Ù‡Ù…Ú¯Ø§Ù…â€ŒØ³Ø§Ø²ÛŒ Ù†ÛŒØ³Øª", c.Number)
		}
		_, err = tx.ExecContext(ctx, "UPDATE invoices SET kind=?,status=?,currency=?,customer_id=?,issuer_profile_id=?,customer_snapshot=?,issuer_snapshot=?,issue_date=?,due_date=?,subtotal_minor=?,discount_minor=?,tax_minor=?,total_minor=?,note=?,updated_at=?,item_discount_minor=?,invoice_discount_mode=?,invoice_discount_value=?,tax_rate_bps=?,numbering_series=?,bank_accounts_snapshot=?,document_type=?,additional_charges_json=?,additional_charge_minor=? WHERE id=?", c.Kind, status, currency, customerID, issuerID, c.CustomerSnapshot, c.IssuerSnapshot, issue, due, sub, itemDiscount+discount, tax, total, c.Note, now, itemDiscount, valueOr(c.Discount.Mode, "fixed"), c.Discount.Value, c.TaxRate, series, c.BankSnapshot, valueOr(c.DocumentType, "sales"), chargeJSON, chargeSum, id)
	} else {
		r, e := tx.ExecContext(ctx, "INSERT INTO invoices(number,kind,status,currency,customer_id,issuer_profile_id,customer_snapshot,issuer_snapshot,issue_date,due_date,subtotal_minor,discount_minor,tax_minor,total_minor,note,created_at,updated_at,item_discount_minor,invoice_discount_mode,invoice_discount_value,tax_rate_bps,numbering_series,bank_accounts_snapshot,document_type,additional_charges_json,additional_charge_minor) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", c.Number, c.Kind, status, currency, customerID, issuerID, c.CustomerSnapshot, c.IssuerSnapshot, issue, due, sub, itemDiscount+discount, tax, total, c.Note, now, now, itemDiscount, valueOr(c.Discount.Mode, "fixed"), c.Discount.Value, c.TaxRate, series, c.BankSnapshot, valueOr(c.DocumentType, "sales"), chargeJSON, chargeSum)
		if e != nil {
			return e
		}
		id, err = r.LastInsertId()
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM invoice_items WHERE invoice_id=?", id); err != nil {
		return err
	}
	for pos, it := range c.Items {
		if _, err = tx.ExecContext(ctx, "INSERT INTO invoice_items(invoice_id,position,description,quantity,unit,unit_price,discount_minor,line_total) VALUES(?,?,?,?,?,?,?,?)", id, pos, it.Description, it.Quantity, it.Unit, it.Price, it.Discount, it.Quantity*it.Price-it.Discount); err != nil {
			return err
		}
	}
	return saveSyncMap(ctx, tx, d.Kind, d.ID, id, d.Version, d.Data)
}
func datePart(v string) string {
	if len(v) >= 10 {
		return v[:10]
	}
	return ""
}
func lookupLinkedCustomer(ctx context.Context, tx *sql.Tx, id int64, snapshot string) int64 {
	var x struct {
		Name  string `json:"Name"`
		Phone string `json:"Phone"`
		Email string `json:"Email"`
	}
	_ = json.Unmarshal([]byte(snapshot), &x)
	var n int64
	if x.Phone != "" || x.Email != "" {
		if tx.QueryRowContext(ctx, "SELECT id FROM customers WHERE (?<>'' AND phone=?) OR (?<>'' AND email=?) ORDER BY id LIMIT 1", x.Phone, x.Phone, x.Email, x.Email).Scan(&n) == nil {
			return n
		}
	}
	if x.Name != "" && tx.QueryRowContext(ctx, "SELECT id FROM customers WHERE name=? ORDER BY id LIMIT 1", x.Name).Scan(&n) == nil {
		return n
	}
	if id > 0 {
		_ = tx.QueryRowContext(ctx, "SELECT id FROM customers WHERE id=?", id).Scan(&n)
	}
	return n
}
func lookupLinkedIssuer(ctx context.Context, tx *sql.Tx, id int64, snapshot string) int64 {
	var x struct {
		Name string `json:"Name"`
	}
	_ = json.Unmarshal([]byte(snapshot), &x)
	var n int64
	if x.Name != "" && tx.QueryRowContext(ctx, "SELECT id FROM issuer_profiles WHERE name=? ORDER BY id LIMIT 1", x.Name).Scan(&n) == nil {
		return n
	}
	if id > 0 {
		_ = tx.QueryRowContext(ctx, "SELECT id FROM issuer_profiles WHERE id=?", id).Scan(&n)
	}
	return n
}
