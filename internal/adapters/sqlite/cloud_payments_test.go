package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestWorkspacePaymentRoundTripAndRemoteReversal(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "payments-sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	docs := []CloudDocument{
		{ID: "11111111111111111111111111111111", Kind: "customer", Version: 1, Data: json.RawMessage(`{"ID":1,"Type":"individual","Name":"Customer","Phone":"09120000000"}`)},
		{ID: "22222222222222222222222222222222", Kind: "issuer", Version: 1, Data: json.RawMessage(`{"ID":1,"Type":"company","Name":"Shop"}`)},
		{ID: "33333333333333333333333333333333", Kind: "invoice", Version: 1, Data: json.RawMessage(`{"ID":1,"Number":"WEB-PAY-1","Kind":"invoice","DocumentType":"sales","Status":"issued","Currency":"IRT","CustomerID":1,"IssuerProfileID":1,"CustomerSnapshot":"{\"Name\":\"Customer\",\"Phone\":\"09120000000\"}","IssuerSnapshot":"{\"Name\":\"Shop\"}","IssueDate":"2026-11-19T00:00:00Z","Items":[{"Description":"Service","Unit":"unit","Quantity":1,"UnitPrice":100,"Discount":0}]}`)},
	}
	if _, err = store.ApplyCloudDocuments(ctx, docs); err != nil {
		t.Fatal(err)
	}
	invoices, err := store.ListInvoiceDocuments(ctx, "")
	if err != nil || len(invoices) != 1 {
		t.Fatalf("invoices=%+v err=%v", invoices, err)
	}
	localID, err := store.RegisterPayment(ctx, domain.Payment{ExternalRef: "LOCAL-REC-1", Amount: 20, Currency: domain.CurrencyToman, PaidAt: time.Now().UTC(), Method: "cash"}, []domain.PaymentAllocation{{InvoiceID: invoices[0].ID, Amount: 20}})
	if err != nil {
		t.Fatal(err)
	}
	exported, err := store.ExportPayments(ctx)
	if err != nil || len(exported) != 1 || len(exported[0].ID) != 32 || exported[0].Allocations[0].DocumentID != docs[2].ID {
		t.Fatalf("export=%+v err=%v", exported, err)
	}
	exportedAgain, err := store.ExportPayments(ctx)
	if err != nil || exportedAgain[0].ID != exported[0].ID {
		t.Fatal("payment cloud ID was not stable")
	}
	remote := SyncedPayment{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ExternalRef: "WEB-REC-1", Amount: 25, Currency: "IRT", PaidAt: time.Now().Unix(), Method: "transfer", Allocations: []SyncedPaymentAllocation{{DocumentID: docs[2].ID, Number: "WEB-PAY-1", Amount: 25}}}
	changed, err := store.ApplyPayments(ctx, []SyncedPayment{remote})
	if err != nil || changed != 1 {
		t.Fatalf("import changed=%d err=%v", changed, err)
	}
	changed, err = store.ApplyPayments(ctx, []SyncedPayment{remote})
	if err != nil || changed != 0 {
		t.Fatalf("repeat import changed=%d err=%v", changed, err)
	}
	var paid int64
	if err = store.db.QueryRowContext(ctx, "SELECT SUM(pa.amount_minor) FROM payment_allocations pa JOIN payments p ON p.id=pa.payment_id").Scan(&paid); err != nil || paid != 45 {
		t.Fatalf("paid=%d err=%v", paid, err)
	}
	reversedAt := time.Now().Unix()
	remote.ReversedAt = &reversedAt
	remote.ReversalReason = "correction"
	changed, err = store.ApplyPayments(ctx, []SyncedPayment{remote})
	if err != nil || changed != 1 {
		t.Fatalf("reverse changed=%d err=%v", changed, err)
	}
	if err = store.db.QueryRowContext(ctx, "SELECT SUM(pa.amount_minor) FROM payment_allocations pa JOIN payments p ON p.id=pa.payment_id LEFT JOIN cloud_payment_reversals r ON r.local_payment_id=p.id WHERE r.local_payment_id IS NULL").Scan(&paid); err != nil || paid != 20 {
		t.Fatalf("remaining paid=%d err=%v", paid, err)
	}
	dashboard, err := store.Dashboard(ctx)
	if err != nil || dashboard.Toman.Received != 20 || dashboard.Toman.Outstanding != 80 {
		t.Fatalf("reversed receipt remained in dashboard: %+v err=%v", dashboard, err)
	}
	var rawPaymentCount, rawAllocationCount int
	if err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments").Scan(&rawPaymentCount); err != nil || rawPaymentCount != 2 {
		t.Fatalf("reversed payment record missing: count=%d err=%v", rawPaymentCount, err)
	}
	if err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_allocations").Scan(&rawAllocationCount); err != nil || rawAllocationCount != 2 {
		t.Fatalf("original allocations missing: count=%d err=%v", rawAllocationCount, err)
	}
	var auditCount int
	if err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE entity_type='payment' AND entity_id=? AND action='cloud_reversed'", localID+1).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("reversal audit=%d err=%v", auditCount, err)
	}
	exportedAfterReverse, err := store.ExportPayments(ctx)
	if err != nil || len(exportedAfterReverse) != 1 || exportedAfterReverse[0].ExternalRef != "LOCAL-REC-1" {
		t.Fatalf("reversed receipt should remain archived locally but not be re-uploaded: %+v err=%v", exportedAfterReverse, err)
	}
	newReversedAt := time.Now().Unix()
	newlyReversed := SyncedPayment{ID: "cccccccccccccccccccccccccccccccc", ExternalRef: "WEB-REC-2", Amount: 15, Currency: "IRT", PaidAt: time.Now().Unix(), ReversedAt: &newReversedAt, ReversalReason: "already reversed online", Allocations: []SyncedPaymentAllocation{{DocumentID: docs[2].ID, Number: "WEB-PAY-1", Amount: 15}}}
	changed, err = store.ApplyPayments(ctx, []SyncedPayment{newlyReversed})
	if err != nil || changed != 1 {
		t.Fatalf("import already-reversed payment changed=%d err=%v", changed, err)
	}
	if err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments").Scan(&rawPaymentCount); err != nil || rawPaymentCount != 3 {
		t.Fatalf("already-reversed payment history missing: count=%d err=%v", rawPaymentCount, err)
	}
	changed, err = store.ApplyPayments(ctx, []SyncedPayment{newlyReversed})
	if err != nil || changed != 0 {
		t.Fatalf("repeat already-reversed import changed=%d err=%v", changed, err)
	}
}
