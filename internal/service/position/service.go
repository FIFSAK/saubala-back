package position

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	domain "github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements the positions (warehouse batches) use cases.
type Service struct {
	positions domain.Repository
	brands    brand.Repository
	receipts  receipt.Repository
	releases  release.Repository
	contracts contract.Repository
}

func NewService(
	positions domain.Repository,
	brands brand.Repository,
	receipts receipt.Repository,
	releases release.Repository,
	contracts contract.Repository,
) *Service {
	return &Service{
		positions: positions,
		brands:    brands,
		receipts:  receipts,
		releases:  releases,
		contracts: contracts,
	}
}

// CreateInput is the payload for creating a position.
type CreateInput struct {
	Name          string
	BrandID       string
	ContractName  string
	ExpiryDate    time.Time
	LotNumber     string
	PurchasePrice int64
	Quantity      int // optional opening stock
	MassGrams     int
	CreatedBy     string
}

// UpdateInput carries the optionally-updated descriptive fields. Quantity is
// intentionally absent: stock changes only via receipts and releases.
type UpdateInput struct {
	Name          *string
	BrandID       *string
	ContractName  *string
	ExpiryDate    *time.Time
	LotNumber     *string
	PurchasePrice *int64
	MassGrams     *int
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Position, error) {
	p, err := domain.New(in.Name, in.BrandID, in.ContractName, in.LotNumber,
		in.ExpiryDate, in.PurchasePrice, in.Quantity, in.MassGrams)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.ensureBrandExists(ctx, p.BrandID); err != nil {
		return nil, err
	}

	// Persist the position with zero stock; opening stock is recorded as a receipt.
	opening := p.Quantity
	p.Quantity = 0
	if err := s.positions.Create(ctx, p); err != nil {
		return nil, err
	}

	if opening > 0 {
		rec, err := receipt.New(time.Now().UTC(), "opening balance", in.CreatedBy,
			[]receipt.Line{{PositionID: p.ID, Quantity: opening}})
		if err != nil {
			_ = s.positions.Delete(ctx, p.ID)
			return nil, err
		}
		// Apply stock first, then write the ledger entry: this ordering guarantees
		// that a persisted receipt always corresponds to applied stock (no orphan).
		if err := s.positions.IncrementQuantity(ctx, p.ID, opening); err != nil {
			_ = s.positions.Delete(ctx, p.ID)
			return nil, err
		}
		if err := s.receipts.Create(ctx, rec); err != nil {
			_ = s.positions.IncrementQuantity(ctx, p.ID, -opening)
			_ = s.positions.Delete(ctx, p.ID)
			return nil, err
		}
		p.Quantity = opening
	}

	return p, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Position, error) {
	p, err := s.positions.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "позиция не найдена")
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Position, int64, error) {
	return s.positions.List(ctx, f)
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*domain.Position, error) {
	p, err := s.positions.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "позиция не найдена")
	}

	if in.Name != nil {
		if *in.Name == "" {
			return nil, web.BadRequest("название позиции обязательно")
		}
		p.Name = *in.Name
	}
	if in.BrandID != nil {
		if err := s.ensureBrandExists(ctx, *in.BrandID); err != nil {
			return nil, err
		}
		p.BrandID = *in.BrandID
	}
	if in.ContractName != nil {
		p.ContractName = *in.ContractName
	}
	if in.ExpiryDate != nil {
		if in.ExpiryDate.IsZero() {
			return nil, web.BadRequest("срок годности обязателен")
		}
		p.ExpiryDate = in.ExpiryDate.UTC()
	}
	if in.LotNumber != nil {
		if *in.LotNumber == "" {
			return nil, web.BadRequest("номер партии обязателен")
		}
		p.LotNumber = *in.LotNumber
	}
	if in.PurchasePrice != nil {
		if *in.PurchasePrice < 0 {
			return nil, web.BadRequest("цена закупки должна быть >= 0")
		}
		p.PurchasePrice = *in.PurchasePrice
	}
	if in.MassGrams != nil {
		if *in.MassGrams < 0 {
			return nil, web.BadRequest("масса (г) должна быть >= 0")
		}
		p.MassGrams = *in.MassGrams
	}

	if err := s.positions.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.positions.GetByID(ctx, id); err != nil {
		return mapNotFound(err, "позиция не найдена")
	}

	recs, err := s.receipts.ListByPosition(ctx, id)
	if err != nil {
		return err
	}
	if len(recs) > 0 {
		return web.Conflict("позиция используется в поступлениях")
	}

	rels, err := s.releases.ListByPosition(ctx, id)
	if err != nil {
		return err
	}
	if len(rels) > 0 {
		return web.Conflict("позиция используется в отгрузках")
	}

	refCount, err := s.contracts.CountByPosition(ctx, id)
	if err != nil {
		return err
	}
	if refCount > 0 {
		return web.Conflict("позиция используется в договорах")
	}

	return s.positions.Delete(ctx, id)
}

// Movements returns the combined stock history (receipts +, releases -) for a
// position, ordered by date ascending.
func (s *Service) Movements(ctx context.Context, id string) ([]domain.Movement, error) {
	if _, err := s.positions.GetByID(ctx, id); err != nil {
		return nil, mapNotFound(err, "позиция не найдена")
	}

	recs, err := s.receipts.ListByPosition(ctx, id)
	if err != nil {
		return nil, err
	}
	rels, err := s.releases.ListByPosition(ctx, id)
	if err != nil {
		return nil, err
	}

	movements := make([]domain.Movement, 0, len(recs)+len(rels))
	for _, r := range recs {
		for _, l := range r.Lines {
			if l.PositionID == id {
				movements = append(movements, domain.Movement{
					Date:        r.Date,
					Type:        domain.MovementReceipt,
					Quantity:    l.Quantity,
					ReferenceID: r.ID,
					Note:        r.Note,
				})
			}
		}
	}
	for _, rel := range rels {
		for _, l := range rel.Lines {
			if l.PositionID == id {
				movements = append(movements, domain.Movement{
					Date:        rel.Date,
					Type:        domain.MovementRelease,
					Quantity:    -l.Quantity,
					ReferenceID: rel.ID,
					Note:        rel.Note,
				})
			}
		}
	}

	sort.SliceStable(movements, func(i, j int) bool {
		return movements[i].Date.Before(movements[j].Date)
	})
	return movements, nil
}

func (s *Service) ensureBrandExists(ctx context.Context, brandID string) error {
	if _, err := s.brands.GetByID(ctx, brandID); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return web.BadRequest("бренд не существует")
		}
		return err
	}
	return nil
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, store.ErrorNotFound) {
		return web.NotFound(msg)
	}
	return err
}
