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
		return nil, fmt.Errorf("договор обязателен")
	}
	if date.IsZero() {
		return nil, fmt.Errorf("дата обязательна")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("требуется хотя бы одна строка")
	}
	for i, l := range lines {
		if strings.TrimSpace(l.ContractLineID) == "" {
			return nil, fmt.Errorf("строка %d: строка договора обязательна", i+1)
		}
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("строка %d: позиция обязательна", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("строка %d: количество должно быть > 0", i+1)
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
