package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Line is a single stock-out row of a release. PositionID is the batch actually
// drawn from (which may be an analogue different from the planned position),
// while ContractLineID ties the draw back to a planned contract line (empty for
// releases without a contract).
type Line struct {
	ContractLineID string
	PositionID     string
	Quantity       int
	UnitCost       int64 // per unit cost at release time (= position purchase price), tiyn
	UnitPrice      int64 // per unit sale price at release time, tiyn; 0 for free releases
}

// Release is an outbound stock operation, either against a contract or a free
// (бесплатная) one without a contract. Creating a release atomically decrements
// stock of the referenced positions. The waybill header data (document number,
// recipient, sender organization) is captured at release time so the waybill can
// be generated later without re-entering it.
type Release struct {
	ID               string
	ContractID       string // empty for free releases
	Date             time.Time
	Note             string
	DocumentNumber   string // «номер накладной»
	RecipientName    string
	RecipientAddress string
	OrganizationID   string // sender organization printed on the waybill
	Lines            []Line
	CreatedBy        string
	CreatedAt        time.Time
}

// NewInput carries the validated-on-construction fields of a release.
type NewInput struct {
	ContractID       string
	Date             time.Time
	Note             string
	DocumentNumber   string
	RecipientName    string
	RecipientAddress string
	OrganizationID   string
	CreatedBy        string
	Lines            []Line
}

// New constructs a validated release. Unit costs and prices are filled in by the
// service from the positions/contract lines being drawn at release time.
func New(in NewInput) (*Release, error) {
	contractID := strings.TrimSpace(in.ContractID)
	if in.Date.IsZero() {
		return nil, fmt.Errorf("дата обязательна")
	}
	if len(in.Lines) == 0 {
		return nil, fmt.Errorf("требуется хотя бы одна строка")
	}
	for i, l := range in.Lines {
		if contractID != "" && strings.TrimSpace(l.ContractLineID) == "" {
			return nil, fmt.Errorf("строка %d: строка договора обязательна", i+1)
		}
		if contractID == "" && strings.TrimSpace(l.ContractLineID) != "" {
			return nil, fmt.Errorf("строка %d: строка договора указана для отпуска без договора", i+1)
		}
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("строка %d: позиция обязательна", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("строка %d: количество должно быть > 0", i+1)
		}
	}

	return &Release{
		ID:               uuid.NewString(),
		ContractID:       contractID,
		Date:             in.Date.UTC(),
		Note:             strings.TrimSpace(in.Note),
		DocumentNumber:   strings.TrimSpace(in.DocumentNumber),
		RecipientName:    strings.TrimSpace(in.RecipientName),
		RecipientAddress: strings.TrimSpace(in.RecipientAddress),
		OrganizationID:   strings.TrimSpace(in.OrganizationID),
		Lines:            in.Lines,
		CreatedBy:        in.CreatedBy,
		CreatedAt:        time.Now().UTC(),
	}, nil
}
