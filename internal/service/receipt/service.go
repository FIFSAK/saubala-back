package receipt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/FIFSAK/saubala-back/internal/domain/position"
	domain "github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/pkg/log"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements inbound stock operations (receipts).
type Service struct {
	receipts  domain.Repository
	positions position.Repository
}

func NewService(receipts domain.Repository, positions position.Repository) *Service {
	return &Service{receipts: receipts, positions: positions}
}

// LineInput is one stock-in row.
type LineInput struct {
	PositionID string
	Quantity   int
}

// CreateInput is the payload for creating a receipt.
type CreateInput struct {
	Date      time.Time
	Note      string
	Lines     []LineInput
	CreatedBy string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.Receipt, error) {
	lines := make([]domain.Line, len(in.Lines))
	for i, l := range in.Lines {
		lines[i] = domain.Line{PositionID: l.PositionID, Quantity: l.Quantity}
	}

	rec, err := domain.New(in.Date, in.Note, in.CreatedBy, lines)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}

	// All referenced positions must exist before any stock is touched.
	for _, l := range rec.Lines {
		if _, err := s.positions.GetByID(ctx, l.PositionID); err != nil {
			if errors.Is(err, store.ErrorNotFound) {
				return nil, web.BadRequest(fmt.Sprintf("позиция %s не существует", l.PositionID))
			}
			return nil, err
		}
	}

	// Apply increments atomically per line, compensating on any failure.
	applied := make([]domain.Line, 0, len(rec.Lines))
	for _, l := range rec.Lines {
		if err := s.positions.IncrementQuantity(ctx, l.PositionID, l.Quantity); err != nil {
			s.compensate(ctx, applied)
			return nil, err
		}
		applied = append(applied, l)
	}

	if err := s.receipts.Create(ctx, rec); err != nil {
		s.compensate(ctx, applied)
		return nil, err
	}
	return rec, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Receipt, error) {
	r, err := s.receipts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("поступление не найдено")
		}
		return nil, err
	}
	return r, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.Receipt, int64, error) {
	return s.receipts.List(ctx, f)
}

// PositionRef is the label data of a referenced position.
type PositionRef struct {
	Name      string
	LotNumber string
}

// PositionRefs batch-loads the position labels referenced by the given
// receipts, so responses carry human-readable names instead of bare IDs.
func (s *Service) PositionRefs(ctx context.Context, recs []domain.Receipt) (map[string]PositionRef, error) {
	ids := make(map[string]struct{})
	for i := range recs {
		for _, l := range recs[i].Lines {
			ids[l.PositionID] = struct{}{}
		}
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	positions, err := s.positions.GetByIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	refs := make(map[string]PositionRef, len(positions))
	for i := range positions {
		refs[positions[i].ID] = PositionRef{Name: positions[i].Name, LotNumber: positions[i].LotNumber}
	}
	return refs, nil
}

// compensate reverses previously-applied stock increments. It runs on a detached
// context so a cancelled request cannot defeat the reversing writes, and logs any
// failure so the (rare) desync is detectable/reconcilable.
func (s *Service) compensate(ctx context.Context, applied []domain.Line) {
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := log.FromContext(ctx)
	for _, l := range applied {
		if err := s.positions.IncrementQuantity(cctx, l.PositionID, -l.Quantity); err != nil {
			logger.Error("stock compensation failed; position quantity may be out of sync",
				zap.String("position_id", l.PositionID),
				zap.Int("delta", -l.Quantity),
				zap.Error(err))
		}
	}
}
