package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestApplyCloudDocumentsMaterializesWorkspaceInvoice(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	docs := []CloudDocument{
		{ID: "customer-cloud-1", Kind: "customer", Version: 1, Data: json.RawMessage(`{"ID":1,"Type":"individual","Name":"مشتری وب","Phone":"09120000000"}`)},
		{ID: "issuer-cloud-1", Kind: "issuer", Version: 1, Data: json.RawMessage(`{"ID":1,"Type":"company","Name":"فروشگاه وب","LegalName":"فروشگاه وب","BankDetails":[{"BankName":"بانک نمونه","AccountNumber":"123"}]}`)},
		{ID: "invoice-cloud-1", Kind: "invoice", Version: 1, Data: json.RawMessage(`{"ID":1,"Number":"WEB-001","Kind":"invoice","DocumentType":"sales","Status":"issued","Currency":"IRT","CustomerID":1,"IssuerProfileID":1,"CustomerSnapshot":"{\"Name\":\"مشتری وب\",\"Phone\":\"09120000000\"}","IssuerSnapshot":"{\"Name\":\"فروشگاه وب\"}","IssueDate":"2026-11-19T00:00:00Z","Items":[{"Description":"خدمت وب","Unit":"عدد","Quantity":2,"UnitPrice":100,"Discount":20}],"InvoiceDiscount":{"Mode":"percent","Value":1000},"TaxRateBPS":900,"AdditionalCharges":[{"Description":"shipping","Amount":5}],"Numbering":{"Series":"invoice"}}`)},
	}
	if _, err = store.ApplyCloudDocumentsAtLocalCursor(ctx, docs, 1); err == nil {
		t.Fatal("expected stale local cursor to stop remote apply")
	}
	var count int
	if err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices").Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale apply changed local data: count=%d err=%v", count, err)
	}
	changed, err := store.ApplyCloudDocuments(ctx, docs)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 {
		t.Fatalf("changed=%d", changed)
	}
	items, err := store.ListInvoiceDocuments(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Number != "WEB-001" || items[0].Total != 182 {
		t.Fatalf("imported invoice=%+v", items)
	}
	var subtotal, itemDiscount, discount, tax, total, charges int64
	if err := store.db.QueryRowContext(ctx, "SELECT subtotal_minor,item_discount_minor,discount_minor,tax_minor,total_minor,additional_charge_minor FROM invoices WHERE number='WEB-001'").Scan(&subtotal, &itemDiscount, &discount, &tax, &total, &charges); err != nil {
		t.Fatal(err)
	}
	if subtotal != 200 || itemDiscount != 20 || discount != 38 || tax != 15 || total != 177 || charges != 5 {
		t.Fatalf("web financial fields lost or rounded differently: subtotal=%d item_discount=%d discount=%d tax=%d total=%d charges=%d", subtotal, itemDiscount, discount, tax, total, charges)
	}
	changed, err = store.ApplyCloudDocuments(ctx, docs)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 {
		t.Fatalf("repeat changed=%d", changed)
	}
	items, err = store.ListInvoiceDocuments(ctx, "")
	if err != nil || len(items) != 1 {
		t.Fatalf("repeat created duplicates: %+v err=%v", items, err)
	}
	deleted, err := store.ApplyCloudDocuments(ctx, []CloudDocument{{ID: "invoice-cloud-1", Kind: "invoice", Version: 2, Deleted: true, Data: json.RawMessage(`{}`)}})
	if err != nil || deleted != 1 {
		t.Fatalf("tombstone apply count=%d err=%v", deleted, err)
	}
	items, err = store.ListInvoiceDocuments(ctx, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("cloud tombstone did not remove local invoice: %+v err=%v", items, err)
	}
	localChanges, _, _, err := store.PendingLocalCloudChanges(ctx, 0, 100)
	if err != nil || len(localChanges) != 0 {
		t.Fatalf("remote apply echoed as local changes: %+v err=%v", localChanges, err)
	}
}

func TestExpensesSyncBothDirections(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	remote := CloudDocument{ID: "expense-cloud", Kind: "expense", Version: 1, Data: json.RawMessage(`{"Amount":120000,"Currency":"IRT","Date":"2026-11-19","Category":"Office","Note":"paper"}`)}
	changed, err := store.ApplyCloudDocuments(ctx, []CloudDocument{remote})
	if err != nil || changed != 1 {
		t.Fatalf("apply remote expense changed=%d err=%v", changed, err)
	}
	rows, err := store.ListExpenses(ctx)
	if err != nil || len(rows) != 1 || int64(rows[0].Amount) != 120000 || rows[0].Category != "Office" {
		t.Fatalf("expense rows=%+v err=%v", rows, err)
	}
	changes, _, _, err := store.PendingLocalCloudChanges(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range changes {
		if d.Kind == "expense" {
			t.Fatal("remote apply echoed an expense as a local edit")
		}
	}
	if _, err = store.SaveExpense(ctx, domain.Expense{Amount: 125000, Currency: domain.CurrencyToman, Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Category: "Travel"}); err != nil {
		t.Fatal(err)
	}
	changes, _, _, err = store.PendingLocalCloudChanges(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range changes {
		if d.Kind == "expense" {
			found = true
			var data map[string]any
			if json.Unmarshal(d.Data, &data) != nil || data["Category"] != "Travel" {
				t.Fatalf("expense payload=%s", d.Data)
			}
		}
	}
	if !found {
		t.Fatal("local expense was not queued for cloud sync")
	}
	if _, err = store.ApplyCloudDocuments(ctx, []CloudDocument{{ID: "expense-cloud", Kind: "expense", Version: 2, Deleted: true, Data: json.RawMessage(`{}`)}}); err != nil {
		t.Fatal(err)
	}
	rows, err = store.ListExpenses(ctx)
	if err != nil || len(rows) != 1 || rows[0].Category != "Travel" {
		t.Fatalf("cloud tombstone removed unexpected expense rows=%+v err=%v", rows, err)
	}
}

func TestCloudPrintTemplateRoundTripsBetweenWorkspaceAndDesktop(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "print-template-sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	imageData := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6//8/IAAA//8GBgMAt2YRIQAAAABJRU5ErkJggg=="
	remote := CloudDocument{ID: "template-web-1", Kind: "template", Version: 3, Data: json.RawMessage(`{"Name":"سربرگ فروش وب","PageSize":"Letter","Orientation":"landscape","FontFamily":"IRANSansX","FontSize":12,"PrimaryColor":"#192631","AccentColor":"#7058ED","ShowLogo":false,"ShowTax":true,"ShowBank":false,"ShowSeal":true,"ShowSignature":true,"ThankYou":"سپاس","LogoData":"` + imageData + `","SealData":"` + imageData + `","SignatureData":"` + imageData + `"}`)}
	changed, err := store.ApplyCloudDocuments(ctx, []CloudDocument{remote})
	if err != nil || changed != 1 {
		t.Fatalf("apply template changed=%d err=%v", changed, err)
	}
	templates, err := store.ListDocumentTemplates(ctx)
	if err != nil || len(templates) != 1 {
		t.Fatalf("templates=%+v err=%v", templates, err)
	}
	got := templates[0]
	if got.Name != "سربرگ فروش وب" || got.Prefix != "FACT-" || got.ThankYou != "سپاس" || got.LogoData != imageData || got.SealData != imageData || got.SignatureData != imageData || !got.ShowSeal || !got.ShowSignature || got.PageSize != "Letter" || got.Orientation != "landscape" || got.FontSize != 12 || got.ShowLogo || !got.ShowTax || got.ShowBankAccounts {
		t.Fatalf("web print options were not applied: %+v", got)
	}

	got.ShowLogo = true
	got.ShowBankAccounts = true
	if _, err = store.SaveDocumentTemplate(ctx, got); err != nil {
		t.Fatal(err)
	}
	docs, _, _, err := store.PendingLocalCloudChanges(ctx, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range docs {
		if doc.Kind != "template" {
			continue
		}
		if doc.ID != remote.ID || doc.Version != remote.Version {
			t.Fatalf("template mapping/version lost: %+v", doc)
		}
		var base cloudTemplate
		if err = json.Unmarshal(doc.BaseData, &base); err != nil || base.LogoData != imageData || base.SealData != imageData || base.SignatureData != imageData {
			t.Fatalf("last common document snapshot missing template assets: %+v err=%v", base, err)
		}
		var data cloudTemplate
		if err = json.Unmarshal(doc.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data.Name != got.Name || data.Prefix != "FACT-" || data.ThankYou != "سپاس" || data.LogoData != imageData || data.SealData != imageData || data.SignatureData != imageData || data.ShowSeal == nil || !*data.ShowSeal || data.ShowSignature == nil || !*data.ShowSignature || data.ShowLogo == nil || !*data.ShowLogo || data.ShowBank == nil || !*data.ShowBank {
			t.Fatalf("desktop template export lost settings: %+v", data)
		}
		got.LogoData = ""
		if _, err = store.SaveDocumentTemplate(ctx, got); err != nil {
			t.Fatal(err)
		}
		deletedImageDocs, _, _, err := store.PendingLocalCloudChanges(ctx, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, deletedImageDoc := range deletedImageDocs {
			if deletedImageDoc.Kind != "template" || deletedImageDoc.ID != remote.ID {
				continue
			}
			var payload map[string]json.RawMessage
			if err = json.Unmarshal(deletedImageDoc.Data, &payload); err != nil {
				t.Fatal(err)
			}
			var logo string
			if err = json.Unmarshal(payload["LogoData"], &logo); err != nil || logo != "" {
				t.Fatalf("cleared image must be explicitly sent to cloud; value=%q err=%v", logo, err)
			}
			return
		}
		t.Fatal("cleared template image update was not queued for cloud sync")
	}
	t.Fatal("desktop template update was not queued for cloud sync")
}

func TestCloudDocumentCursorPersistsAndNeverMovesBackward(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cursor.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveCloudDocumentCursor(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveCloudDocumentCursor(ctx, 17); err != nil {
		t.Fatal(err)
	}
	cursor, err := store.CloudDocumentCursor(ctx)
	if err != nil || cursor != 42 {
		t.Fatalf("cursor=%d err=%v", cursor, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cursor, err = store.CloudDocumentCursor(ctx)
	if err != nil || cursor != 42 {
		t.Fatalf("reopened cursor=%d err=%v", cursor, err)
	}
}

func TestLocalChangesTrackProfilesAndTombstonesWithoutRemoteEcho(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "local-changes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result, err := store.db.ExecContext(ctx, "INSERT INTO customers(name,phone,created_at,updated_at) VALUES('مشتری محلی','09120000000','2026-11-19','2026-11-19')")
	if err != nil {
		t.Fatal(err)
	}
	localID, _ := result.LastInsertId()
	if _, err = store.db.ExecContext(ctx, "INSERT INTO customer_phones(customer_id,phone,position) VALUES(?,?,0)", localID, "09121111111"); err != nil {
		t.Fatal(err)
	}
	docs, cursor, more, err := store.PendingLocalCloudChanges(ctx, 0, 100)
	if err != nil || more || len(docs) != 1 || docs[0].Kind != "customer" || docs[0].LocalID != localID || docs[0].Version != 0 {
		t.Fatalf("docs=%+v cursor=%d more=%v err=%v", docs, cursor, more, err)
	}
	var customer cloudCustomer
	if err = json.Unmarshal(docs[0].Data, &customer); err != nil || customer.Name != "مشتری محلی" || len(customer.Phones) != 2 {
		t.Fatalf("customer=%+v err=%v", customer, err)
	}
	if err = store.AcknowledgeLocalCloudDocument(ctx, docs[0], 1); err != nil {
		t.Fatal(err)
	}
	var baseSnapshot string
	if err = store.db.QueryRowContext(ctx, "SELECT base_data FROM cloud_document_map WHERE kind='customer' AND local_id=?", localID).Scan(&baseSnapshot); err != nil || baseSnapshot != string(docs[0].Data) {
		t.Fatalf("acknowledged snapshot=%q want=%q err=%v", baseSnapshot, docs[0].Data, err)
	}
	if err = store.SaveLocalCloudPushCursor(ctx, cursor); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, "UPDATE customers SET name='ویرایش محلی' WHERE id=?", localID); err != nil {
		t.Fatal(err)
	}
	docs, cursor, more, err = store.PendingLocalCloudChanges(ctx, cursor, 100)
	if err != nil || more || len(docs) != 1 || docs[0].Version != 1 {
		t.Fatalf("update docs=%+v cursor=%d more=%v err=%v", docs, cursor, more, err)
	}
	if err = store.AcknowledgeLocalCloudDocument(ctx, docs[0], 2); err != nil {
		t.Fatal(err)
	}
	if err = store.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM local_cloud_changes").Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, "DELETE FROM customers WHERE id=?", localID); err != nil {
		t.Fatal(err)
	}
	docs, cursor, more, err = store.PendingLocalCloudChanges(ctx, cursor, 100)
	if err != nil || more || len(docs) != 1 || !docs[0].Deleted || docs[0].Version != 2 {
		t.Fatalf("delete docs=%+v cursor=%d more=%v err=%v", docs, cursor, more, err)
	}
}

func TestSeedDesktopCloudMappingsMatchesServerIdentity(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "seed-maps.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result, err := store.db.ExecContext(ctx, "INSERT INTO customers(name,created_at,updated_at) VALUES('مشتری','now','now')")
	if err != nil {
		t.Fatal(err)
	}
	localID, _ := result.LastInsertId()
	if _, err = store.db.ExecContext(ctx, "INSERT INTO cloud_document_map(kind,cloud_id,local_id,remote_version,updated_at,base_data) VALUES('customer','old-cloud',?,12,'now','{\"old\":true}')", localID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, `INSERT INTO document_templates(name,page_size,orientation,font_family,font_size,primary_color,accent_color,created_at,updated_at)
		VALUES('Bootstrap template','A4','portrait','IRANSansX',10,'#192631','#7058ED','now','now')`); err != nil {
		t.Fatal(err)
	}
	baselineCursor, err := store.LatestLocalCloudChangeID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	userID := strings.Repeat("ab", 16)
	if err = store.SeedDesktopCloudMappings(ctx, userID); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("hesora-desktop:%s:customer:%d", userID, localID)))
	want := hex.EncodeToString(sum[:16])
	var got string
	if err = store.db.QueryRowContext(ctx, "SELECT cloud_id FROM cloud_document_map WHERE kind='customer' AND local_id=?", localID).Scan(&got); err != nil || got != want {
		t.Fatalf("cloud id=%s want=%s err=%v", got, want, err)
	}
	var base string
	var version int64
	if err = store.db.QueryRowContext(ctx, "SELECT base_data,remote_version FROM cloud_document_map WHERE kind='customer' AND local_id=?", localID).Scan(&base, &version); err != nil || base != "" || version != 0 {
		t.Fatalf("new account mapping must discard prior base/version; base=%q version=%d err=%v", base, version, err)
	}
	seedCursor, err := store.LatestLocalCloudChangeID(ctx)
	if err != nil || seedCursor <= baselineCursor {
		t.Fatalf("template seed cursor=%d baseline=%d err=%v", seedCursor, baselineCursor, err)
	}
	queued, _, more, err := store.PendingLocalCloudChanges(ctx, baselineCursor, 20)
	if err != nil || more || len(queued) != 1 || queued[0].Kind != "template" {
		t.Fatalf("seeded template should remain pending after backup boundary: docs=%+v more=%v err=%v", queued, more, err)
	}
}

func TestLocalInvoiceExportIncludesCurrentLinesAndFinancialFields(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "local-invoice-export.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.DB().ExecContext(ctx, `INSERT INTO customers(name,created_at,updated_at) VALUES('Buyer','now','now');
		INSERT INTO issuer_profiles(name,created_at,updated_at) VALUES('Shop','now','now');
		INSERT INTO invoices(number,kind,status,currency,customer_id,issuer_profile_id,customer_snapshot,issuer_snapshot,issue_date,subtotal_minor,discount_minor,tax_minor,total_minor,created_at,updated_at,invoice_discount_mode,invoice_discount_value,tax_rate_bps,additional_charges_json)
		VALUES('FACT-LOCAL-1','invoice','issued','IRT',1,1,'{"Name":"Buyer"}','{"Name":"Shop"}','2026-11-19',100,0,10,110,'now','now','fixed',0,1000,'[{"Description":"shipping","Amount":10}]');
		INSERT INTO invoice_items(invoice_id,position,description,quantity,unit,unit_price,discount_minor,line_total) VALUES(1,0,'Item',1,'unit',100,0,100);`)
	if err != nil {
		t.Fatal(err)
	}
	docs, _, _, err := store.PendingLocalCloudChanges(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, doc := range docs {
		if doc.Kind != "invoice" {
			continue
		}
		var invoice cloudInvoice
		if err = json.Unmarshal(doc.Data, &invoice); err != nil {
			t.Fatal(err)
		}
		if invoice.Number == "FACT-LOCAL-1" {
			found = true
			if len(invoice.Items) != 1 || invoice.Items[0].Description != "Item" || len(invoice.Charges) != 1 || invoice.Charges[0].Amount != 10 || invoice.TaxRate != 1000 {
				t.Fatalf("invoice export lost data: %+v", invoice)
			}
		}
	}
	if !found {
		t.Fatal("local invoice missing from change feed")
	}
}
