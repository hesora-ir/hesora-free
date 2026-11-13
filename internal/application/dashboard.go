package application

import (
	"context"
	"github.com/hesora-ir/hesora-free/internal/domain"
)

type DashboardStore interface {
	Dashboard(context.Context) (domain.DashboardSnapshot, error)
}

type Dashboard struct{ store DashboardStore }

func NewDashboard(store DashboardStore) *Dashboard { return &Dashboard{store: store} }

func (s *Dashboard) Load(ctx context.Context) (domain.DashboardSnapshot, error) {
	return s.store.Dashboard(ctx)
}
