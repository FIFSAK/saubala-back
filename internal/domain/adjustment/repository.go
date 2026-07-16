package adjustment

import "context"

// Repository is the persistence contract for stock adjustments.
type Repository interface {
	Create(ctx context.Context, a *Adjustment) error
	// ListByPosition returns every adjustment recorded for the given position
	// (used to build the position movement history).
	ListByPosition(ctx context.Context, positionID string) ([]Adjustment, error)
}
