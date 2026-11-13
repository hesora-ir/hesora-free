package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/hesora-ir/hesora-free/internal/application"
	"github.com/hesora-ir/hesora-free/internal/domain"
)

func TestDashboardEmpty(t *testing.T) {
	store := openTestStore(t)
	got, err := application.NewDashboard(store).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Customers != 0 || got.Unsettled != 0 || got.Settled != 0 || got.Toman != (domain.DashboardAmounts{}) || got.Rial != (domain.DashboardAmounts{}) || len(got.Activities) != 0 {
		t.Fatalf("empty dashboard: %+v", got)
	}
}

func TestDashboardLifecycleAndCurrencies(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if _, err := store.CreateCustomer(ctx, domain.Customer{Name: "مشتری داشبورد"}); err != nil {
		t.Fatal(err)
	}
	create := func(currency domain.Currency, kind, status string, total domain.Money) int64 {
		t.Helper()
		doc := testInvoiceDraft()
		doc.Currency, doc.Kind, doc.Items[0].UnitPrice = currency, kind, total
		id, _, err := store.CreateInvoiceDraft(ctx, doc)
		if err != nil {
			t.Fatal(err)
		}
		if status == "issued" {
			if err := store.IssueInvoice(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		if status == "void" {
			if _, err := store.DB().ExecContext(ctx, "UPDATE invoices SET status='void' WHERE id=?", id); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	pay := func(id int64, amount domain.Money, currency domain.Currency) {
		t.Helper()
		_, err := store.RegisterPayment(ctx, domain.Payment{Amount: amount, Currency: currency, PaidAt: time.Now()}, []domain.PaymentAllocation{{InvoiceID: id, Amount: amount}})
		if err != nil {
			t.Fatal(err)
		}
	}
	partial := create(domain.CurrencyToman, "invoice", "issued", 1000)
	pay(partial, 200, domain.CurrencyToman)
	pay(partial, 300, domain.CurrencyToman)
	settled := create(domain.CurrencyToman, "invoice", "issued", 700)
	pay(settled, 700, domain.CurrencyToman)
	rial := create(domain.CurrencyRial, "invoice", "issued", 9000)
	pay(rial, 2000, domain.CurrencyRial)
	create(domain.CurrencyToman, "invoice", "issued", 0)
	create(domain.CurrencyToman, "invoice", "draft", 50000)
	create(domain.CurrencyRial, "invoice", "void", 80000)
	create(domain.CurrencyToman, "proforma", "draft", 60000)
	got, err := store.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Customers != 1 || got.Unsettled != 2 || got.Settled != 2 {
		t.Fatalf("counts: %+v", got)
	}
	if got.Toman.Received != 1200 || got.Toman.Outstanding != 500 || got.Rial.Received != 2000 || got.Rial.Outstanding != 7000 {
		t.Fatalf("amounts: %+v", got)
	}
	if len(got.Activities) != 8 {
		t.Fatalf("activities = %d, want 8", len(got.Activities))
	}
	for i, event := range got.Activities {
		if event.Number == "" || event.OccurredAt.IsZero() {
			t.Fatalf("invalid event: %+v", event)
		}
		if i > 0 && event.OccurredAt.After(got.Activities[i-1].OccurredAt) {
			t.Fatal("activities are not newest first")
		}
	}
	// A further payment must update both the receipt sum and settlement counts.
	pay(partial, 500, domain.CurrencyToman)
	got, err = store.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unsettled != 1 || got.Settled != 3 || got.Toman.Outstanding != 0 || got.Toman.Received != 1700 || got.Activities[0].Action != "payment_allocated" {
		t.Fatalf("after settlement: %+v", got)
	}
}

func TestDashboardCountsBeyondListLimit(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, err := store.DB().ExecContext(ctx, `WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<510)
		INSERT INTO invoices(number,kind,status,currency,issue_date,subtotal_minor,total_minor,created_at,updated_at)
		SELECT 'INV-'||x,'invoice','issued','IRT','2026-10-30',1,1,'2026-10-30','2026-10-30' FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unsettled != 510 || got.Toman.Outstanding != 510 {
		t.Fatalf("truncated aggregate: %+v", got)
	}
}
