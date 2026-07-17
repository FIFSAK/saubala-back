package org

import "context"

// Repository is the persistence port for sender organizations. The collection
// is small (a handful of firms), so List returns everything without pagination.
type Repository interface {
	Create(ctx context.Context, o *Organization) error
	GetByID(ctx context.Context, id string) (*Organization, error)
	// GetByIDs returns the organizations with the given IDs; unknown IDs are
	// skipped silently (used for batch reference-label lookups).
	GetByIDs(ctx context.Context, ids []string) ([]Organization, error)
	Update(ctx context.Context, o *Organization) error
	Delete(ctx context.Context, id string) error
	// List returns all organizations ordered by creation time.
	List(ctx context.Context) ([]Organization, error)
	Count(ctx context.Context) (int64, error)
}
