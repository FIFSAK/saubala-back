package release

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/domain/org"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	domain "github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/log"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements outbound stock operations: releases against a contract and
// free (бесплатные) releases without one.
type Service struct {
	releases  domain.Repository
	contracts contract.Repository
	positions position.Repository
	orgs      org.Repository
}

func NewService(releases domain.Repository, contracts contract.Repository, positions position.Repository, orgs org.Repository) *Service {
	return &Service{releases: releases, contracts: contracts, positions: positions, orgs: orgs}
}

// LineInput is one stock-out row. ContractLineID is required for contract
// releases and must be empty for free ones.
type LineInput struct {
	ContractLineID string
	PositionID     string
	Quantity       int
}

// CreateInput is the payload for creating a release. The waybill header data
// (document number, recipient, sender organization) is captured here so the
// waybill can be generated later without re-entering it.
type CreateInput struct {
	ContractID       string // empty for a free release
	Date             time.Time
	Note             string
	DocumentNumber   string
	RecipientName    string
	RecipientAddress string
	OrganizationID   string
	Lines            []LineInput
	CreatedBy        string
}

// Create performs a release. For contract releases it validates the contract and
// its lines and enforces the plan limit; for free releases it only checks the
// positions. It then atomically decrements stock (never going negative) with
// compensation if the persist step fails. Because dev MongoDB is typically a
// standalone node without multi-document transactions, multi-line atomicity is
// achieved with per-line atomic $inc plus compensation.
//
// Note: the stock decrement is atomic and can never go negative, but the plan
// limit (released_so_far + requested <= planned) is a read-then-write check and
// is therefore NOT concurrency-atomic — two simultaneous releases against the
// same contract line could jointly exceed the plan. This is an accepted MVP
// limitation: per the spec, full transactional atomicity is deferred to a
// replica-set deployment (session.WithTransaction); a per-line atomic released
// counter would be the standalone-friendly hardening if it becomes necessary.
func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Release, error) {
	lines := make([]domain.Line, len(in.Lines))
	for i, l := range in.Lines {
		lines[i] = domain.Line{ContractLineID: l.ContractLineID, PositionID: l.PositionID, Quantity: l.Quantity}
	}

	rel, err := domain.New(domain.NewInput{
		ContractID:       in.ContractID,
		Date:             in.Date,
		Note:             in.Note,
		DocumentNumber:   in.DocumentNumber,
		RecipientName:    in.RecipientName,
		RecipientAddress: in.RecipientAddress,
		OrganizationID:   in.OrganizationID,
		CreatedBy:        in.CreatedBy,
		Lines:            lines,
	})
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}

	if rel.OrganizationID != "" {
		if _, err := s.orgs.GetByID(ctx, rel.OrganizationID); err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest("организация-отправитель не существует")
			}
			return nil, err
		}
	}

	// plannedPrice maps contract line id -> planned sale price (contract releases).
	plannedPrice := make(map[string]*int64)
	planned := make(map[string]int)
	if rel.ContractID != "" {
		c, err := s.contracts.GetByID(ctx, rel.ContractID)
		if err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.NotFound("договор не найден")
			}
			return nil, err
		}
		for _, cl := range c.Lines {
			planned[cl.ID] = cl.PlannedQuantity
			plannedPrice[cl.ID] = cl.PlannedPrice
		}
	}

	// Validate contract-line references, load positions, capture unit costs and
	// sale prices, and accumulate the quantity requested per contract line.
	requested := make(map[string]int)
	for i := range rel.Lines {
		l := &rel.Lines[i]
		p, err := s.positions.GetByID(ctx, l.PositionID)
		if err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest(fmt.Sprintf("позиция %s не существует", l.PositionID))
			}
			return nil, err
		}
		l.UnitCost = p.PurchasePrice
		if rel.ContractID != "" {
			if _, ok := planned[l.ContractLineID]; !ok {
				return nil, web.BadRequest(fmt.Sprintf("строка договора %s не принадлежит этому договору", l.ContractLineID))
			}
			l.UnitPrice = UnitPrice(plannedPrice[l.ContractLineID], p.PurchasePrice)
			requested[l.ContractLineID] += l.Quantity
		}
		// Free releases keep UnitPrice = 0: they are shipped free of charge.
	}

	// Plan control: already-released + requested must not exceed the plan.
	if rel.ContractID != "" {
		releasedSoFar, err := s.releases.ReleasedByContract(ctx, rel.ContractID)
		if err != nil {
			return nil, err
		}
		for lineID, reqQty := range requested {
			if releasedSoFar[lineID]+reqQty > planned[lineID] {
				return nil, web.Unprocessable(fmt.Sprintf(
					"отгрузка превышает план по строке договора %s (план %d, уже отгружено %d, запрошено %d)",
					lineID, planned[lineID], releasedSoFar[lineID], reqQty))
			}
		}
	}

	// Stock control: atomic decrement that never goes negative, with compensation.
	applied := make([]domain.Line, 0, len(rel.Lines))
	for _, l := range rel.Lines {
		ok, err := s.positions.DecrementIfAvailable(ctx, l.PositionID, l.Quantity)
		if err != nil {
			s.compensate(ctx, applied)
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest(fmt.Sprintf("позиция %s не существует", l.PositionID))
			}
			return nil, err
		}
		if !ok {
			s.compensate(ctx, applied)
			return nil, web.Conflict(fmt.Sprintf("недостаточно остатка для позиции %s", l.PositionID))
		}
		applied = append(applied, l)
	}

	if err := s.releases.Create(ctx, rel); err != nil {
		s.compensate(ctx, applied)
		return nil, err
	}
	return rel, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Release, error) {
	r, err := s.releases.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("отгрузка не найдена")
		}
		return nil, err
	}
	return r, nil
}

// UpdateWaybill updates the waybill header fields of a release (document
// number, recipient, sender organization). Stock and lines are immutable —
// this exists so waybill data entered at download time for older releases is
// remembered instead of being asked again.
func (s *Service) UpdateWaybill(ctx context.Context, id string, u domain.WaybillUpdate) (*domain.Release, error) {
	trim := func(p *string) {
		if p != nil {
			*p = strings.TrimSpace(*p)
		}
	}
	trim(u.DocumentNumber)
	trim(u.RecipientName)
	trim(u.RecipientAddress)
	trim(u.OrganizationID)

	if u.OrganizationID != nil && *u.OrganizationID != "" {
		if _, err := s.orgs.GetByID(ctx, *u.OrganizationID); err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest("организация-отправитель не существует")
			}
			return nil, err
		}
	}

	if err := s.releases.UpdateWaybill(ctx, id, u); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("отгрузка не найдена")
		}
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Release, int64, error) {
	return s.releases.List(ctx, f)
}

// ContractRef is the label data of a referenced contract.
type ContractRef struct {
	Number string
	Name   string
}

// PositionRef is the label data of a referenced position.
type PositionRef struct {
	Name      string
	LotNumber string
}

// Refs is the batch-loaded reference data of a set of releases: contract and
// position labels, sender organization names, and the pricing fallbacks used to
// value lines persisted before sale prices were stored (planned contract-line
// prices and position purchase prices).
type Refs struct {
	Contracts     map[string]ContractRef
	Positions     map[string]PositionRef
	Organizations map[string]string // organization id -> name
	plannedPrice  map[string]*int64 // contract line id -> planned price
	purchasePrice map[string]int64  // position id -> purchase price
}

// Refs batch-loads the reference data of the given releases, so responses carry
// human-readable names instead of bare IDs and every line can be priced.
func (s *Service) Refs(ctx context.Context, rels []domain.Release) (*Refs, error) {
	contractIDs := make(map[string]struct{})
	positionIDs := make(map[string]struct{})
	orgIDs := make(map[string]struct{})
	for i := range rels {
		if rels[i].ContractID != "" {
			contractIDs[rels[i].ContractID] = struct{}{}
		}
		if rels[i].OrganizationID != "" {
			orgIDs[rels[i].OrganizationID] = struct{}{}
		}
		for _, l := range rels[i].Lines {
			positionIDs[l.PositionID] = struct{}{}
		}
	}

	contracts, err := s.contracts.GetByIDs(ctx, keys(contractIDs))
	if err != nil {
		return nil, err
	}
	positions, err := s.positions.GetByIDs(ctx, keys(positionIDs))
	if err != nil {
		return nil, err
	}
	orgs, err := s.orgs.GetByIDs(ctx, keys(orgIDs))
	if err != nil {
		return nil, err
	}

	refs := &Refs{
		Contracts:     make(map[string]ContractRef, len(contracts)),
		Positions:     make(map[string]PositionRef, len(positions)),
		Organizations: make(map[string]string, len(orgs)),
		plannedPrice:  make(map[string]*int64),
		purchasePrice: make(map[string]int64, len(positions)),
	}
	for i := range contracts {
		refs.Contracts[contracts[i].ID] = ContractRef{Number: contracts[i].ContractNumber, Name: contracts[i].Name}
		for _, cl := range contracts[i].Lines {
			refs.plannedPrice[cl.ID] = cl.PlannedPrice
		}
	}
	for i := range positions {
		refs.Positions[positions[i].ID] = PositionRef{Name: positions[i].Name, LotNumber: positions[i].LotNumber}
		refs.purchasePrice[positions[i].ID] = positions[i].PurchasePrice
	}
	for i := range orgs {
		refs.Organizations[orgs[i].ID] = orgs[i].Name
	}
	return refs, nil
}

// LineUnitPrice returns the effective sale price of a release line: the price
// stored at release time when present, else (for contract releases persisted
// before prices were stored) the planned contract-line price falling back to the
// position purchase price. Free-release lines are always 0.
func (r *Refs) LineUnitPrice(l domain.Line) int64 {
	if l.UnitPrice > 0 {
		return l.UnitPrice
	}
	if l.ContractLineID == "" {
		return 0
	}
	purchase := r.purchasePrice[l.PositionID]
	if purchase == 0 {
		purchase = l.UnitCost
	}
	return UnitPrice(r.plannedPrice[l.ContractLineID], purchase)
}

// Amount returns the total sale value («сумма отгрузки») of a release.
func (r *Refs) Amount(rel *domain.Release) int64 {
	var sum int64
	for _, l := range rel.Lines {
		sum += r.LineUnitPrice(l) * int64(l.Quantity)
	}
	return sum
}

// UnitPrice picks the sale price of a contract-release line: the planned
// contract-line price when present, else the position purchase price.
func UnitPrice(planned *int64, purchasePrice int64) int64 {
	if planned != nil {
		return *planned
	}
	return purchasePrice
}

func keys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// compensate restores stock previously decremented in a failed release. It runs
// on a detached context so a cancelled request cannot also defeat the restoring
// writes, and logs any failure so the (rare) desync is detectable/reconcilable.
func (s *Service) compensate(ctx context.Context, applied []domain.Line) {
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := log.FromContext(ctx)
	for _, l := range applied {
		if err := s.positions.IncrementQuantity(cctx, l.PositionID, l.Quantity); err != nil {
			logger.Error("stock compensation failed; position quantity may be out of sync",
				zap.String("position_id", l.PositionID),
				zap.Int("delta", l.Quantity),
				zap.Error(err))
		}
	}
}
