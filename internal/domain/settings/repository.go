package settings

import "context"

// Repository is the persistence port for the organization settings singleton.
type Repository interface {
	// Get returns the singleton settings document, or store.ErrorNotFound if it
	// has not been seeded yet.
	Get(ctx context.Context) (*Organization, error)
	// Upsert writes the singleton settings document, creating it if absent.
	Upsert(ctx context.Context, o *Organization) error
}
