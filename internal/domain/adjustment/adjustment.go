package adjustment

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Adjustment is a manual stock correction applied to a single position. Unlike
// receipts (bare inbound) and releases (contract-bound outbound), an adjustment
// is a bare signed delta recorded when an operator edits a position's quantity
// directly. A positive delta raises stock, a negative delta lowers it, so the
// combined receipts/releases/adjustments ledger reconciles with the stored
// position quantity.
type Adjustment struct {
	ID         string
	PositionID string
	Delta      int // signed change in units; never zero
	Note       string
	CreatedBy  string
	CreatedAt  time.Time
}

// New constructs a validated adjustment with a generated ID and timestamp. delta
// must be non-zero (a no-op edit records nothing).
func New(positionID string, delta int, note, createdBy string) (*Adjustment, error) {
	positionID = strings.TrimSpace(positionID)
	if positionID == "" {
		return nil, fmt.Errorf("позиция обязательна")
	}
	if delta == 0 {
		return nil, fmt.Errorf("изменение количества не может быть нулевым")
	}

	return &Adjustment{
		ID:         uuid.NewString(),
		PositionID: positionID,
		Delta:      delta,
		Note:       strings.TrimSpace(note),
		CreatedBy:  createdBy,
		CreatedAt:  time.Now().UTC(),
	}, nil
}
