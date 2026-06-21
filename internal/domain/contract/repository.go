package contract

import (
	"context"
	"time"
)

// Filter describes the query parameters accepted by the contracts list endpoint.
type Filter struct {
	Q        string // matched against name / contract_number
	BIN      string
	DateFrom *time.Time
	DateTo   *time.Time
	Page     int
	PageSize int
	Sort     string
	Order    string
}

// Repository is the persistence contract for contracts.
type Repository interface {
	Create(ctx context.Context, c *Contract) error
	GetByID(ctx context.Context, id string) (*Contract, error)
	GetByNumber(ctx context.Context, number string) (*Contract, error)
	Update(ctx context.Context, c *Contract) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f Filter) ([]Contract, int64, error)
	// CountByPosition reports how many contracts reference the given position in
	// any appendix line (used to block position deletion).
	CountByPosition(ctx context.Context, positionID string) (int64, error)
}
