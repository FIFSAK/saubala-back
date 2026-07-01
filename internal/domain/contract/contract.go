package contract

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var binRe = regexp.MustCompile(`^\d{12}$`)

// Line is a planned row of a contract's appendix: how much of a given position is
// planned to be released over the contract period.
type Line struct {
	ID              string
	PositionID      string
	PlannedQuantity int
	PlannedPrice    *int64 // per unit, tiyn; optional
}

// Contract is a yearly plan of what should be released/written off. Creating a
// contract does not touch stock — releases draw against it over time.
type Contract struct {
	ID              string
	Name            string
	CustomerAddress string
	ContractNumber  string
	ContractDate    time.Time
	BIN             string
	Lines           []Line
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// New constructs a validated contract, generating the contract ID and a stable
// ID for every appendix line.
func New(name, customerAddress, contractNumber string, contractDate time.Time, bin string, createdBy string, lines []Line) (*Contract, error) {
	if err := validateHeader(name, customerAddress, contractNumber, contractDate, bin); err != nil {
		return nil, err
	}
	normalized, err := NormalizeLines(lines)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Contract{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(name),
		CustomerAddress: strings.TrimSpace(customerAddress),
		ContractNumber:  strings.TrimSpace(contractNumber),
		ContractDate:    contractDate.UTC(),
		BIN:             strings.TrimSpace(bin),
		Lines:           normalized,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func validateHeader(name, customerAddress, contractNumber string, contractDate time.Time, bin string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("наименование обязательно")
	}
	if strings.TrimSpace(customerAddress) == "" {
		return fmt.Errorf("адрес заказчика обязателен")
	}
	if strings.TrimSpace(contractNumber) == "" {
		return fmt.Errorf("номер договора обязателен")
	}
	if contractDate.IsZero() {
		return fmt.Errorf("дата договора обязательна")
	}
	if err := ValidateBIN(bin); err != nil {
		return err
	}
	return nil
}

// ValidateBIN checks that bin is exactly 12 digits.
func ValidateBIN(bin string) error {
	if !binRe.MatchString(strings.TrimSpace(bin)) {
		return fmt.Errorf("БИН должен состоять ровно из 12 цифр")
	}
	return nil
}

// NormalizeLines validates the appendix lines and assigns an ID to any line that
// does not already have one (preserving existing IDs across updates).
func NormalizeLines(lines []Line) ([]Line, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("требуется хотя бы одна строка")
	}
	out := make([]Line, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l.PositionID) == "" {
			return nil, fmt.Errorf("строка %d: позиция обязательна", i+1)
		}
		if l.PlannedQuantity <= 0 {
			return nil, fmt.Errorf("строка %d: плановое количество должно быть > 0", i+1)
		}
		if l.PlannedPrice != nil && *l.PlannedPrice < 0 {
			return nil, fmt.Errorf("строка %d: плановая цена должна быть >= 0", i+1)
		}
		id := strings.TrimSpace(l.ID)
		if id == "" {
			id = uuid.NewString()
		}
		out[i] = Line{
			ID:              id,
			PositionID:      strings.TrimSpace(l.PositionID),
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return out, nil
}
