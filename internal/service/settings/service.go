// Package settings implements the use cases around the settings singleton
// (invoice defaults): reading it, updating it, and seeding its defaults on
// startup.
package settings

import (
	"context"
	"errors"
	"time"

	domain "github.com/FIFSAK/saubala-back/internal/domain/settings"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service manages the settings singleton.
type Service struct {
	settings domain.Repository
}

func NewService(settings domain.Repository) *Service {
	return &Service{settings: settings}
}

// Get returns the current settings. It falls back to the seeded defaults if the
// document is somehow missing, so callers always get a usable value.
func (s *Service) Get(ctx context.Context) (*domain.Settings, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return domain.Default(), nil
		}
		return nil, err
	}
	return set, nil
}

// Update validates and persists the settings.
func (s *Service) Update(ctx context.Context, set *domain.Settings) (*domain.Settings, error) {
	set.Normalize()
	if err := set.Validate(); err != nil {
		return nil, web.BadRequest(err.Error())
	}
	set.UpdatedAt = time.Now().UTC()
	if err := s.settings.Upsert(ctx, set); err != nil {
		return nil, err
	}
	return set, nil
}

// EnsureDefault seeds the settings singleton with the customer's current values
// if it does not exist yet. It is idempotent and safe to run on every startup.
func (s *Service) EnsureDefault(ctx context.Context) error {
	_, err := s.settings.Get(ctx)
	if err == nil {
		return nil // already present
	}
	if !errors.Is(err, store.ErrorNotFound) {
		return err
	}
	return s.settings.Upsert(ctx, domain.Default())
}
