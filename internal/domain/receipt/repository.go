package receipt

import (
	"context"
	"time"
)

// Filter describes the query parameters accepted by the receipts list endpoint.
type Filter struct {
	PositionID string
	SupplierID string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	Sort       string
	Order      string
}

// Repository is the persistence contract for receipts.
type Repository interface {
	Create(ctx context.Context, r *Receipt) error
	GetByID(ctx context.Context, id string) (*Receipt, error)
	List(ctx context.Context, f Filter) ([]Receipt, int64, error)
	// ListByPosition returns every receipt that references the given position in
	// any of its lines (used to build the position movement history).
	ListByPosition(ctx context.Context, positionID string) ([]Receipt, error)
	// CountBySupplier reports how many receipts reference the given supplier
	// (used to block supplier deletion).
	CountBySupplier(ctx context.Context, supplierID string) (int64, error)
	// InvoiceTotalBySupplier returns the summed invoice amounts (tiyn) per
	// supplier id for the given suppliers in one query (used on the suppliers
	// reference to show how much has been purchased from each).
	InvoiceTotalBySupplier(ctx context.Context, supplierIDs []string) (map[string]int64, error)
}
