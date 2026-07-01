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
	ID        string
	Date      time.Time
	Note      string
	Lines     []Line
	CreatedBy string
	CreatedAt time.Time
}

// New constructs a validated receipt with a generated ID and timestamp.
func New(date time.Time, note, createdBy string, lines []Line) (*Receipt, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("дата обязательна")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("требуется хотя бы одна строка")
	}
	for i, l := range lines {
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("строка %d: позиция обязательна", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("строка %d: количество должно быть > 0", i+1)
		}
	}

	return &Receipt{
		ID:        uuid.NewString(),
		Date:      date.UTC(),
		Note:      strings.TrimSpace(note),
		Lines:     lines,
		CreatedBy: createdBy,
		CreatedAt: time.Now().UTC(),
	}, nil
}
