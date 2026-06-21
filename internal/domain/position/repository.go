package position

import (
	"context"
	"time"
)

// Filter describes the query parameters accepted by the positions list endpoint.
type Filter struct {
	Q            string // matched against name / contract_name / supplier_name / lot_number
	BrandID      string
	ExpiryBefore *time.Time
	ExpiryAfter  *time.Time
	InStock      bool // when true, only positions with quantity > 0
	Page         int
	PageSize     int
	Sort         string
	Order        string
}

// Repository is the persistence contract for positions.
//
// Stock quantity is never written through Update; it changes only via the
// atomic IncrementQuantity (receipts) and DecrementIfAvailable (releases) paths.
type Repository interface {
	Create(ctx context.Context, p *Position) error
	GetByID(ctx context.Context, id string) (*Position, error)
	// Update persists descriptive fields only (it must not change Quantity).
	Update(ctx context.Context, p *Position) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f Filter) ([]Position, int64, error)
	CountByBrand(ctx context.Context, brandID string) (int64, error)

	// IncrementQuantity atomically adds delta (may be negative for compensation)
	// to a position's stock.
	IncrementQuantity(ctx context.Context, id string, delta int) error
	// DecrementIfAvailable atomically subtracts qty only if the current stock is
	// at least qty. It reports ok=false (without error) when the position exists
	// but stock is insufficient, and returns store.ErrorNotFound when the position
	// does not exist.
	DecrementIfAvailable(ctx context.Context, id string, qty int) (ok bool, err error)
}
