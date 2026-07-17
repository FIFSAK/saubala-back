package position

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/adjustment"
	"github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	domain "github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/internal/domain/supplier"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements the positions (warehouse batches) use cases.
type Service struct {
	positions   domain.Repository
	brands      brand.Repository
	suppliers   supplier.Repository
	receipts    receipt.Repository
	releases    release.Repository
	contracts   contract.Repository
	adjustments adjustment.Repository
}

func NewService(
	positions domain.Repository,
	brands brand.Repository,
	suppliers supplier.Repository,
	receipts receipt.Repository,
	releases release.Repository,
	contracts contract.Repository,
	adjustments adjustment.Repository,
) *Service {
	return &Service{
		positions:   positions,
		brands:      brands,
		suppliers:   suppliers,
		receipts:    receipts,
		releases:    releases,
		contracts:   contracts,
		adjustments: adjustments,
	}
}

// CreateInput is the payload for creating a position.
type CreateInput struct {
	Name          string
	BrandID       string
	SupplierID    string // optional
	ContractName  string
	ExpiryDate    time.Time
	LotNumber     string
	PurchasePrice int64
	Quantity      int // optional opening stock
	MassGrams     int
	CreatedBy     string
}

// UpdateInput carries the optionally-updated descriptive fields. When Quantity
// is set, the difference from current stock is recorded as an adjustment ledger
// entry (a receipt/release replacement for bare manual corrections), so stock
// stays reconciled with the movement history. ActorID attributes that entry.
type UpdateInput struct {
	Name          *string
	BrandID       *string
	SupplierID    *string
	ContractName  *string
	ExpiryDate    *time.Time
	LotNumber     *string
	PurchasePrice *int64
	MassGrams     *int
	Quantity      *int
	ActorID       string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Position, error) {
	p, err := domain.New(in.Name, in.BrandID, in.SupplierID, in.ContractName, in.LotNumber,
		in.ExpiryDate, in.PurchasePrice, in.Quantity, in.MassGrams)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.ensureBrandExists(ctx, p.BrandID); err != nil {
		return nil, err
	}
	if err := s.ensureSupplierExists(ctx, p.SupplierID); err != nil {
		return nil, err
	}

	// Persist the position with zero stock; opening stock is recorded as a receipt.
	opening := p.Quantity
	p.Quantity = 0
	if err := s.positions.Create(ctx, p); err != nil {
		return nil, err
	}

	if opening > 0 {
		// The opening receipt inherits the position's supplier.
		rec, err := receipt.New(time.Now().UTC(), "opening balance", p.SupplierID, 0, in.CreatedBy,
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

// BrandNames batch-loads brand names for the given positions (including
// soft-deleted brands, so existing references keep their labels).
func (s *Service) BrandNames(ctx context.Context, ps []domain.Position) (map[string]string, error) {
	ids := make(map[string]struct{}, len(ps))
	for i := range ps {
		ids[ps[i].BrandID] = struct{}{}
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	brands, err := s.brands.GetByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(brands))
	for i := range brands {
		names[brands[i].ID] = brands[i].Name
	}
	return names, nil
}

// SupplierNames batch-loads supplier names for the given positions.
func (s *Service) SupplierNames(ctx context.Context, ps []domain.Position) (map[string]string, error) {
	ids := make(map[string]struct{}, len(ps))
	for i := range ps {
		if ps[i].SupplierID != "" {
			ids[ps[i].SupplierID] = struct{}{}
		}
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	suppliers, err := s.suppliers.GetByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(suppliers))
	for i := range suppliers {
		names[suppliers[i].ID] = suppliers[i].Name
	}
	return names, nil
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
	if in.SupplierID != nil {
		if err := s.ensureSupplierExists(ctx, *in.SupplierID); err != nil {
			return nil, err
		}
		p.SupplierID = *in.SupplierID
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

	// A quantity edit is a manual stock correction: record it through the ledger
	// (adjustment entry) rather than a direct write, so movements reconcile.
	if in.Quantity != nil {
		if *in.Quantity < 0 {
			return nil, web.BadRequest("количество должно быть >= 0")
		}
		if delta := *in.Quantity - p.Quantity; delta != 0 {
			if err := s.applyAdjustment(ctx, p.ID, delta, in.ActorID); err != nil {
				return nil, err
			}
			p.Quantity = *in.Quantity
		}
	}
	return p, nil
}

// applyAdjustment records a manual stock correction of delta units for a
// position. It applies the stock change first and then writes the adjustment
// ledger entry — the same ordering as Create's opening balance — so a persisted
// entry always corresponds to applied stock, rolling the stock change back if
// the ledger write fails.
func (s *Service) applyAdjustment(ctx context.Context, positionID string, delta int, actorID string) error {
	adj, err := adjustment.New(positionID, delta, "корректировка", actorID)
	if err != nil {
		return web.BadRequest(err.Error())
	}

	if delta > 0 {
		if err := s.positions.IncrementQuantity(ctx, positionID, delta); err != nil {
			return mapNotFound(err, "позиция не найдена")
		}
		if err := s.adjustments.Create(ctx, adj); err != nil {
			_ = s.positions.IncrementQuantity(ctx, positionID, -delta)
			return err
		}
		return nil
	}

	// delta < 0: draw stock down, but never below zero (guards against a
	// concurrent release having already reduced it since we read the position).
	ok, err := s.positions.DecrementIfAvailable(ctx, positionID, -delta)
	if err != nil {
		return mapNotFound(err, "позиция не найдена")
	}
	if !ok {
		return web.Conflict("недостаточно остатка для уменьшения количества")
	}
	if err := s.adjustments.Create(ctx, adj); err != nil {
		_ = s.positions.IncrementQuantity(ctx, positionID, -delta) // add drawn stock back
		return err
	}
	return nil
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
	adjs, err := s.adjustments.ListByPosition(ctx, id)
	if err != nil {
		return nil, err
	}

	movements := make([]domain.Movement, 0, len(recs)+len(rels)+len(adjs))
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
	for _, a := range adjs {
		movements = append(movements, domain.Movement{
			Date:        a.CreatedAt,
			Type:        domain.MovementAdjustment,
			Quantity:    a.Delta,
			ReferenceID: a.ID,
			Note:        a.Note,
		})
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

// ensureSupplierExists validates a supplier reference; an empty id is allowed
// (the supplier link is optional).
func (s *Service) ensureSupplierExists(ctx context.Context, supplierID string) error {
	if supplierID == "" {
		return nil
	}
	if _, err := s.suppliers.GetByID(ctx, supplierID); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return web.BadRequest("поставщик не существует")
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
