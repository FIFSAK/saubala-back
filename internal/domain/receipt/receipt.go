package receipt

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Line is a single stock-in row of a receipt.
type Line struct {
	PositionID string
	Quantity   int
}

// Receipt is an inbound stock operation. Creating a receipt atomically increases
// the quantity of each referenced position.
type Receipt struct {
	ID             string
	Date           time.Time
	Supplier       string
	DocumentNumber string
	Note           string
	Lines          []Line
	CreatedBy      string
	CreatedAt      time.Time
}

// New constructs a validated receipt with a generated ID and timestamp.
func New(date time.Time, supplier, documentNumber, note, createdBy string, lines []Line) (*Receipt, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("date is required")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("at least one line is required")
	}
	for i, l := range lines {
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("line %d: position_id is required", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("line %d: quantity must be > 0", i+1)
		}
	}

	return &Receipt{
		ID:             uuid.NewString(),
		Date:           date.UTC(),
		Supplier:       strings.TrimSpace(supplier),
		DocumentNumber: strings.TrimSpace(documentNumber),
		Note:           strings.TrimSpace(note),
		Lines:          lines,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now().UTC(),
	}, nil
}
