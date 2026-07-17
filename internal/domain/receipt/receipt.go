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
// the quantity of each referenced position. SupplierID and InvoiceAmount
// describe the supplier invoice behind the delivery (both optional).
// Counterparty is the legacy free-text supplier name of receipts persisted
// before the suppliers reference existed; new receipts reference a supplier.
type Receipt struct {
	ID            string
	Date          time.Time
	Note          string
	SupplierID    string
	Counterparty  string // legacy, read-only
	InvoiceAmount int64  // supplier invoice total, tiyn
	Lines         []Line
	CreatedBy     string
	CreatedAt     time.Time
}

// New constructs a validated receipt with a generated ID and timestamp.
func New(date time.Time, note, supplierID string, invoiceAmount int64, createdBy string, lines []Line) (*Receipt, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("дата обязательна")
	}
	if invoiceAmount < 0 {
		return nil, fmt.Errorf("сумма инвойса должна быть >= 0")
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
		ID:            uuid.NewString(),
		Date:          date.UTC(),
		Note:          strings.TrimSpace(note),
		SupplierID:    strings.TrimSpace(supplierID),
		InvoiceAmount: invoiceAmount,
		Lines:         lines,
		CreatedBy:     createdBy,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
