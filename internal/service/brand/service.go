package brand

import (
	"context"
	"errors"

	domain "github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements the brand catalogue with soft deletion and uniqueness.
type Service struct {
	brands    domain.Repository
	positions position.Repository
}

func NewService(brands domain.Repository, positions position.Repository) *Service {
	return &Service{brands: brands, positions: positions}
}

func (s *Service) Create(ctx context.Context, name string) (*domain.Brand, error) {
	b, err := domain.New(name)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.ensureNameAvailable(ctx, b.Name, ""); err != nil {
		return nil, err
	}
	if err := s.brands.Create(ctx, b); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, web.Conflict("a brand with this name already exists")
		}
		return nil, err
	}
	return b, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Brand, error) {
	b, err := s.brands.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "brand not found")
	}
	return b, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Brand, int64, error) {
	return s.brands.List(ctx, f)
}

func (s *Service) Update(ctx context.Context, id, name string) (*domain.Brand, error) {
	b, err := s.brands.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "brand not found")
	}
	if err := domain.ValidateName(name); err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.ensureNameAvailable(ctx, name, id); err != nil {
		return nil, err
	}
	b.Name = name
	if err := s.brands.Update(ctx, b); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, web.Conflict("a brand with this name already exists")
		}
		return nil, err
	}
	return b, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.brands.GetByID(ctx, id); err != nil {
		return mapNotFound(err, "brand not found")
	}
	count, err := s.positions.CountByBrand(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return web.Conflict("brand is referenced by existing positions")
	}
	return s.brands.SoftDelete(ctx, id)
}

// ensureNameAvailable checks that no other active brand already uses name.
func (s *Service) ensureNameAvailable(ctx context.Context, name, excludeID string) error {
	existing, err := s.brands.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil
		}
		return err
	}
	if existing.ID != excludeID {
		return web.Conflict("a brand with this name already exists")
	}
	return nil
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, store.ErrorNotFound) {
		return web.NotFound(msg)
	}
	return err
}
