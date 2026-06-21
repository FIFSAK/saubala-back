package position

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Position is a warehouse lot/batch of a specific product. Each position carries
// its own expiry date, purchase price and remaining stock quantity.
//
// Money fields are stored as int64 tiyn (1 ₸ = 100 tiyn). Mass is grams per unit.
type Position struct {
	ID            string
	Name          string
	BrandID       string
	ContractName  string
	SupplierName  string
	ExpiryDate    time.Time
	LotNumber     string
	PurchasePrice int64 // per unit, tiyn
	Quantity      int   // current stock on hand, units
	MassGrams     int   // per unit, grams
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// New constructs a validated position. initialQuantity may be 0; when > 0 the
// service treats it as an opening receipt.
func New(name, brandID, contractName, supplierName, lotNumber string, expiryDate time.Time, purchasePrice int64, initialQuantity, massGrams int) (*Position, error) {
	name = strings.TrimSpace(name)
	brandID = strings.TrimSpace(brandID)
	lotNumber = strings.TrimSpace(lotNumber)

	if name == "" {
		return nil, fmt.Errorf("position name is required")
	}
	if brandID == "" {
		return nil, fmt.Errorf("brand_id is required")
	}
	if expiryDate.IsZero() {
		return nil, fmt.Errorf("expiry_date is required")
	}
	if lotNumber == "" {
		return nil, fmt.Errorf("lot_number is required")
	}
	if purchasePrice < 0 {
		return nil, fmt.Errorf("purchase_price must be >= 0")
	}
	if massGrams < 0 {
		return nil, fmt.Errorf("mass_grams must be >= 0")
	}
	if initialQuantity < 0 {
		return nil, fmt.Errorf("quantity must be >= 0")
	}

	now := time.Now().UTC()
	return &Position{
		ID:            uuid.NewString(),
		Name:          name,
		BrandID:       brandID,
		ContractName:  strings.TrimSpace(contractName),
		SupplierName:  strings.TrimSpace(supplierName),
		ExpiryDate:    expiryDate.UTC(),
		LotNumber:     lotNumber,
		PurchasePrice: purchasePrice,
		Quantity:      initialQuantity,
		MassGrams:     massGrams,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// MovementType identifies the kind of stock movement.
type MovementType string

const (
	MovementReceipt MovementType = "receipt"
	MovementRelease MovementType = "release"
)

// Movement is a single entry in a position's combined stock history. Receipts
// contribute positive quantities, releases negative ones.
type Movement struct {
	Date        time.Time    `json:"date"`
	Type        MovementType `json:"type"`
	Quantity    int          `json:"quantity"`
	ReferenceID string       `json:"reference_id"`
	Note        string       `json:"note"`
}
