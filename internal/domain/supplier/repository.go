package supplier

import "context"

// Filter describes the query parameters accepted by the suppliers list endpoint.
type Filter struct {
	Q        string // matched against name / bin / country
	Type     Type   // optional
	Page     int
	PageSize int
	Sort     string
	Order    string
}

// Repository is the persistence contract for suppliers.
type Repository interface {
	Create(ctx context.Context, s *Supplier) error
	GetByID(ctx context.Context, id string) (*Supplier, error)
	// GetByIDs returns the suppliers with the given IDs; unknown IDs are skipped
	// silently (used for batch reference-label lookups).
	GetByIDs(ctx context.Context, ids []string) ([]Supplier, error)
	Update(ctx context.Context, s *Supplier) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f Filter) ([]Supplier, int64, error)
}
