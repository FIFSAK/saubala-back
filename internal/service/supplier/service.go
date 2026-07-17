// Package supplier implements the suppliers-reference use cases: CRUD plus the
// purchase totals shown on the reference page.
package supplier

import (
	"context"
	"errors"

	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	domain "github.com/FIFSAK/saubala-back/internal/domain/supplier"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service manages the suppliers reference.
type Service struct {
	suppliers domain.Repository
	positions position.Repository
	receipts  receipt.Repository
}

func NewService(suppliers domain.Repository, positions position.Repository, receipts receipt.Repository) *Service {
	return &Service{suppliers: suppliers, positions: positions, receipts: receipts}
}

// Input carries the editable fields of a supplier.
type Input struct {
	Name    string
	Type    domain.Type
	BIN     string
	Country string
	Phone   string
	Email   string
}

func (s *Service) Create(ctx context.Context, in Input) (*domain.Supplier, error) {
	sup, err := domain.New(in.Name, in.Type, in.BIN, in.Country, in.Phone, in.Email)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.suppliers.Create(ctx, sup); err != nil {
		return nil, err
	}
	return sup, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Supplier, error) {
	sup, err := s.suppliers.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("поставщик не найден")
		}
		return nil, err
	}
	return sup, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Supplier, int64, error) {
	return s.suppliers.List(ctx, f)
}

// InvoiceTotals batch-loads the summed receipt invoice amounts (tiyn) per
// supplier — «на какую сумму закупаюсь у каждого поставщика».
func (s *Service) InvoiceTotals(ctx context.Context, suppliers []domain.Supplier) (map[string]int64, error) {
	ids := make([]string, len(suppliers))
	for i := range suppliers {
		ids[i] = suppliers[i].ID
	}
	return s.receipts.InvoiceTotalBySupplier(ctx, ids)
}

func (s *Service) Update(ctx context.Context, id string, in Input) (*domain.Supplier, error) {
	sup, err := s.suppliers.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("поставщик не найден")
		}
		return nil, err
	}
	sup.Name = in.Name
	sup.Type = in.Type
	sup.BIN = in.BIN
	sup.Country = in.Country
	sup.Phone = in.Phone
	sup.Email = in.Email
	sup.Normalize()
	if err := sup.Validate(); err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.suppliers.Update(ctx, sup); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("поставщик не найден")
		}
		return nil, err
	}
	return sup, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.suppliers.GetByID(ctx, id); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return web.NotFound("поставщик не найден")
		}
		return err
	}
	count, err := s.positions.CountBySupplier(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return web.Conflict("поставщик указан у позиций, его нельзя удалить")
	}
	count, err = s.receipts.CountBySupplier(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return web.Conflict("по поставщику есть поступления, его нельзя удалить")
	}
	return s.suppliers.Delete(ctx, id)
}
