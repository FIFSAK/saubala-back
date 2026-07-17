package release

import (
	"context"
	"time"
)

// Filter describes the query parameters accepted by the releases list endpoint.
type Filter struct {
	ContractID string
	OnlyFree   bool // only releases without a contract (ignored when ContractID is set)
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	Sort       string
	Order      string
}

// WaybillUpdate carries the optionally-updated waybill header fields of a
// release; nil fields are left untouched.
type WaybillUpdate struct {
	DocumentNumber   *string
	RecipientName    *string
	RecipientAddress *string
	OrganizationID   *string
}

// Repository is the persistence contract for releases.
type Repository interface {
	Create(ctx context.Context, r *Release) error
	GetByID(ctx context.Context, id string) (*Release, error)
	// UpdateWaybill sets the provided waybill header fields on a release (used
	// when the data is first entered at download time for older releases).
	UpdateWaybill(ctx context.Context, id string, u WaybillUpdate) error
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
	// ReleasedByContracts returns the total released quantity per contract line
	// id, grouped by contract id, for the given contracts in one query (used to
	// compute fulfilled amounts on contract lists).
	ReleasedByContracts(ctx context.Context, contractIDs []string) (map[string]map[string]int, error)
	// CountByOrganization reports how many releases reference the given sender
	// organization (used to block organization deletion).
	CountByOrganization(ctx context.Context, organizationID string) (int64, error)
}
