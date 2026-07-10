package release

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	domain "github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/log"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements outbound stock operations (releases against a contract).
type Service struct {
	releases  domain.Repository
	contracts contract.Repository
	positions position.Repository
}

func NewService(releases domain.Repository, contracts contract.Repository, positions position.Repository) *Service {
	return &Service{releases: releases, contracts: contracts, positions: positions}
}

// LineInput is one stock-out row.
type LineInput struct {
	ContractLineID string
	PositionID     string
	Quantity       int
}

// CreateInput is the payload for creating a release.
type CreateInput struct {
	ContractID string
	Date       time.Time
	Note       string
	Lines      []LineInput
	CreatedBy  string
}

// Create performs a release against a contract. It validates the contract and
// its lines, enforces the plan limit, then atomically decrements stock (never
// going negative) with compensation if the persist step fails. Because dev
// MongoDB is typically a standalone node without multi-document transactions,
// multi-line atomicity is achieved with per-line atomic $inc plus compensation.
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

	rel, err := domain.New(in.ContractID, in.Date, in.Note, in.CreatedBy, lines)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}

	c, err := s.contracts.GetByID(ctx, rel.ContractID)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("договор не найден")
		}
		return nil, err
	}

	planned := make(map[string]int, len(c.Lines))
	for _, cl := range c.Lines {
		planned[cl.ID] = cl.PlannedQuantity
	}

	// Validate contract-line references, load positions, capture unit costs, and
	// accumulate the quantity requested per contract line in this call.
	requested := make(map[string]int)
	for i := range rel.Lines {
		l := &rel.Lines[i]
		if _, ok := planned[l.ContractLineID]; !ok {
			return nil, web.BadRequest(fmt.Sprintf("строка договора %s не принадлежит этому договору", l.ContractLineID))
		}
		p, err := s.positions.GetByID(ctx, l.PositionID)
		if err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest(fmt.Sprintf("позиция %s не существует", l.PositionID))
			}
			return nil, err
		}
		l.UnitCost = p.PurchasePrice
		requested[l.ContractLineID] += l.Quantity
	}

	// Plan control: already-released + requested must not exceed the plan.
	releasedSoFar, err := s.releases.ReleasedByContract(ctx, c.ID)
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

// Refs batch-loads the contract and position labels referenced by the given
// releases, so responses carry human-readable names instead of bare IDs.
func (s *Service) Refs(ctx context.Context, rels []domain.Release) (map[string]ContractRef, map[string]PositionRef, error) {
	contractIDs := make(map[string]struct{})
	positionIDs := make(map[string]struct{})
	for i := range rels {
		contractIDs[rels[i].ContractID] = struct{}{}
		for _, l := range rels[i].Lines {
			positionIDs[l.PositionID] = struct{}{}
		}
	}

	contracts, err := s.contracts.GetByIDs(ctx, keys(contractIDs))
	if err != nil {
		return nil, nil, err
	}
	positions, err := s.positions.GetByIDs(ctx, keys(positionIDs))
	if err != nil {
		return nil, nil, err
	}

	crefs := make(map[string]ContractRef, len(contracts))
	for i := range contracts {
		crefs[contracts[i].ID] = ContractRef{Number: contracts[i].ContractNumber, Name: contracts[i].Name}
	}
	prefs := make(map[string]PositionRef, len(positions))
	for i := range positions {
		prefs[positions[i].ID] = PositionRef{Name: positions[i].Name, LotNumber: positions[i].LotNumber}
	}
	return crefs, prefs, nil
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
