package brand

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Brand is a catalogue entry for a product brand. Brands are the only entity
// with soft deletion: removed brands keep their document with DeletedAt set.
type Brand struct {
	ID        string
	Name      string
	DeletedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New constructs a validated brand with a generated ID and timestamps.
func New(name string) (*Brand, error) {
	name = strings.TrimSpace(name)
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Brand{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ValidateName checks that a brand name is non-empty.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("brand name is required")
	}
	return nil
}
