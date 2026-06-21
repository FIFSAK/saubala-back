package contract

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements contracts (yearly release plans).
type Service struct {
	contracts domain.Repository
	positions position.Repository
	releases  release.Repository
}

func NewService(contracts domain.Repository, positions position.Repository, releases release.Repository) *Service {
	return &Service{contracts: contracts, positions: positions, releases: releases}
}

// LineInput is one appendix (planned) line.
type LineInput struct {
	ID              string // optional; preserved across updates
	PositionID      string
	PlannedQuantity int
	PlannedPrice    *int64
}

// CreateInput is the payload for creating a contract.
type CreateInput struct {
	Name            string
	CustomerAddress string
	ContractNumber  string
	ContractDate    time.Time
	BIN             string
	Lines           []LineInput
	CreatedBy       string
}

// UpdateInput carries the optionally-updated header and appendix lines.
type UpdateInput struct {
	Name            *string
	CustomerAddress *string
	ContractNumber  *string
	ContractDate    *time.Time
	BIN             *string
	Lines           *[]LineInput
}

// LineProgress is the per-line plan/release progress returned with a contract.
type LineProgress struct {
	Planned   int
	Released  int
	Remaining int
}

func toDomainLines(in []LineInput) []domain.Line {
	lines := make([]domain.Line, len(in))
	for i, l := range in {
		lines[i] = domain.Line{
			ID:              l.ID,
			PositionID:      l.PositionID,
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return lines
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Contract, error) {
	c, err := domain.New(in.Name, in.CustomerAddress, in.ContractNumber, in.ContractDate, in.BIN,
		in.CreatedBy, toDomainLines(in.Lines))
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.validatePositions(ctx, c.Lines); err != nil {
		return nil, err
	}
	if err := s.ensureNumberAvailable(ctx, c.ContractNumber, ""); err != nil {
		return nil, err
	}
	if err := s.contracts.Create(ctx, c); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, web.Conflict("a contract with this number already exists")
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Contract, map[string]LineProgress, error) {
	c, err := s.contracts.GetByID(ctx, id)
	if err != nil {
		return nil, nil, mapNotFound(err, "contract not found")
	}
	progress, err := s.progress(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	return c, progress, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Contract, int64, error) {
	return s.contracts.List(ctx, f)
}

func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*domain.Contract, map[string]LineProgress, error) {
	c, err := s.contracts.GetByID(ctx, id)
	if err != nil {
		return nil, nil, mapNotFound(err, "contract not found")
	}

	if in.Name != nil {
		c.Name = *in.Name
	}
	if in.CustomerAddress != nil {
		c.CustomerAddress = *in.CustomerAddress
	}
	if in.ContractDate != nil {
		c.ContractDate = in.ContractDate.UTC()
	}
	if in.BIN != nil {
		if err := domain.ValidateBIN(*in.BIN); err != nil {
			return nil, nil, web.BadRequest(err.Error())
		}
		c.BIN = *in.BIN
	}
	if in.ContractNumber != nil && *in.ContractNumber != c.ContractNumber {
		if *in.ContractNumber == "" {
			return nil, nil, web.BadRequest("contract_number is required")
		}
		if err := s.ensureNumberAvailable(ctx, *in.ContractNumber, c.ID); err != nil {
			return nil, nil, err
		}
		c.ContractNumber = *in.ContractNumber
	}
	if in.Lines != nil {
		lines, err := domain.NormalizeLines(toDomainLines(*in.Lines))
		if err != nil {
			return nil, nil, web.BadRequest(err.Error())
		}
		if err := s.validatePositions(ctx, lines); err != nil {
			return nil, nil, err
		}
		// Plan-control integrity: any contract line that already has releases must
		// stay present (same id) with planned_quantity >= already-released, else
		// prior releases would be orphaned and the plan limit could be bypassed.
		// Compare against the OLD line ids — NormalizeLines mints fresh ids for
		// blank ones, so the client must echo back ids of already-released lines.
		released, err := s.releases.ReleasedByContract(ctx, c.ID)
		if err != nil {
			return nil, nil, err
		}
		incoming := make(map[string]int, len(lines))
		for _, nl := range lines {
			incoming[nl.ID] = nl.PlannedQuantity
		}
		for _, old := range c.Lines {
			rel := released[old.ID]
			if rel <= 0 {
				continue
			}
			newPlanned, ok := incoming[old.ID]
			if !ok {
				return nil, nil, web.Conflict(fmt.Sprintf("contract line %s has releases and cannot be removed", old.ID))
			}
			if newPlanned < rel {
				return nil, nil, web.Unprocessable(fmt.Sprintf(
					"planned_quantity for contract line %s (%d) is below the already-released quantity (%d)",
					old.ID, newPlanned, rel))
			}
		}
		c.Lines = lines
	}

	// Re-validate the header as a whole (name/address/number/date/bin).
	if _, err := domain.New(c.Name, c.CustomerAddress, c.ContractNumber, c.ContractDate, c.BIN, c.CreatedBy, c.Lines); err != nil {
		return nil, nil, web.BadRequest(err.Error())
	}

	if err := s.contracts.Update(ctx, c); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, nil, web.Conflict("a contract with this number already exists")
		}
		return nil, nil, err
	}

	progress, err := s.progress(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	return c, progress, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.contracts.GetByID(ctx, id); err != nil {
		return mapNotFound(err, "contract not found")
	}
	count, err := s.releases.CountByContract(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return web.Conflict("contract has releases and cannot be deleted")
	}
	return s.contracts.Delete(ctx, id)
}

func (s *Service) progress(ctx context.Context, c *domain.Contract) (map[string]LineProgress, error) {
	released, err := s.releases.ReleasedByContract(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]LineProgress, len(c.Lines))
	for _, l := range c.Lines {
		rel := released[l.ID]
		remaining := l.PlannedQuantity - rel
		if remaining < 0 {
			remaining = 0
		}
		out[l.ID] = LineProgress{Planned: l.PlannedQuantity, Released: rel, Remaining: remaining}
	}
	return out, nil
}

func (s *Service) validatePositions(ctx context.Context, lines []domain.Line) error {
	for _, l := range lines {
		if _, err := s.positions.GetByID(ctx, l.PositionID); err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return web.BadRequest(fmt.Sprintf("position %s does not exist", l.PositionID))
			}
			return err
		}
	}
	return nil
}

func (s *Service) ensureNumberAvailable(ctx context.Context, number, excludeID string) error {
	existing, err := s.contracts.GetByNumber(ctx, number)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil
		}
		return err
	}
	if existing.ID != excludeID {
		return web.Conflict("a contract with this number already exists")
	}
	return nil
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, store.ErrorNotFound) {
		return web.NotFound(msg)
	}
	return err
}
