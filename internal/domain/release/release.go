package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Line is a single stock-out row of a release. PositionID is the batch actually
// drawn from (which may be an analogue different from the planned position),
// while ContractLineID ties the draw back to a planned contract line.
type Line struct {
	ContractLineID string
	PositionID     string
	Quantity       int
	UnitCost       int64 // per unit cost at release time (= position purchase price), tiyn
}

// Release is an outbound stock operation against a contract. Creating a release
// atomically decrements stock of the referenced positions.
type Release struct {
	ID         string
	ContractID string
	Date       time.Time
	Note       string
	Lines      []Line
	CreatedBy  string
	CreatedAt  time.Time
}

// New constructs a validated release. Unit costs are filled in by the service
// from the positions being drawn at release time.
func New(contractID string, date time.Time, note, createdBy string, lines []Line) (*Release, error) {
	if strings.TrimSpace(contractID) == "" {
		return nil, fmt.Errorf("contract_id is required")
	}
	if date.IsZero() {
		return nil, fmt.Errorf("date is required")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("at least one line is required")
	}
	for i, l := range lines {
		if strings.TrimSpace(l.ContractLineID) == "" {
			return nil, fmt.Errorf("line %d: contract_line_id is required", i+1)
		}
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("line %d: position_id is required", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("line %d: quantity must be > 0", i+1)
		}
	}

	return &Release{
		ID:         uuid.NewString(),
		ContractID: strings.TrimSpace(contractID),
		Date:       date.UTC(),
		Note:       strings.TrimSpace(note),
		Lines:      lines,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now().UTC(),
	}, nil
}
