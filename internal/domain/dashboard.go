package domain

import "time"

// DashboardSnapshot keeps currencies separate; no implicit monetary conversion.
type DashboardSnapshot struct {
	Customers  int64
	Unsettled  int64
	Settled    int64
	Toman      DashboardAmounts
	Rial       DashboardAmounts
	Activities []DashboardActivity
}

type DashboardAmounts struct {
	Received    Money
	Outstanding Money
}

type DashboardActivity struct {
	Number     string
	Action     string
	OccurredAt time.Time
}
