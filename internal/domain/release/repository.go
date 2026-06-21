package release

import (
	"context"
	"time"
)

// Filter describes the query parameters accepted by the releases list endpoint.
type Filter struct {
	ContractID string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	Sort       string
	Order      string
}

// Repository is the persistence contract for releases.
type Repository interface {
	Create(ctx context.Context, r *Release) error
	GetByID(ctx context.Context, id string) (*Release, error)
	List(ctx context.Context, f Filter) ([]Release, int64, error)
	// ListByPosition returns every release that references the given position in
	// any of its lines (used to build the position movement history).
	ListByPosition(ctx context.Context, positionID string) ([]Release, error)
	// CountByContract reports how many releases exist for a contract (used to
	// block contract deletion).
	CountByContract(ctx context.Context, contractID string) (int64, error)
	// ReleasedByContract returns the total released quantity per contract line id
	// for the given contract (used for plan-progress and over-release checks).
	ReleasedByContract(ctx context.Context, contractID string) (map[string]int, error)
}
