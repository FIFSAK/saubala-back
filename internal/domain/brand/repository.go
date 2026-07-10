package brand

import "context"

// Filter describes the query parameters accepted by the brands list endpoint.
type Filter struct {
	Q        string // matched against name (case-insensitive)
	Page     int
	PageSize int
	Sort     string
	Order    string
}

// Repository is the persistence contract for brands. All read methods operate
// only on non-deleted brands (DeletedAt == nil), except GetByIDs which includes
// soft-deleted brands so existing references keep their labels.
type Repository interface {
	Create(ctx context.Context, b *Brand) error
	GetByID(ctx context.Context, id string) (*Brand, error)
	// GetByIDs returns the brands with the given IDs (including soft-deleted
	// ones); unknown IDs are skipped silently (used for batch label lookups).
	GetByIDs(ctx context.Context, ids []string) ([]Brand, error)
	GetByName(ctx context.Context, name string) (*Brand, error)
	Update(ctx context.Context, b *Brand) error
	SoftDelete(ctx context.Context, id string) error
	List(ctx context.Context, f Filter) ([]Brand, int64, error)
}
