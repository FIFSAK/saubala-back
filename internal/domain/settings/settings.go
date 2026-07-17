// Package settings holds the organization-wide settings singleton: the invoice
// defaults used when generating release waybills (Форма З-2). The seller
// (sender) organizations themselves live in the org package — there can be
// several of them, chosen per release.
package settings

import (
	"fmt"
	"strings"
	"time"
)

// ID is the fixed identifier of the singleton settings document.
const ID = "org"

// Settings is the singleton settings document carrying the invoice defaults
// (VAT rate, line-description prefix, unit of measure).
//
// It is not user-scoped: exactly one document exists, seeded on startup and
// edited from the settings page.
type Settings struct {
	VATRatePercent        int    // НДС, %, prices are treated as VAT-inclusive
	LineDescriptionPrefix string // prefix wrapping each position name in column «Наименование»
	DefaultUnit           string // «Единица измерения», e.g. штука
	UpdatedAt             time.Time
}

// Default returns the settings seeded on first startup, pre-filled with the
// customer's current values.
func Default() *Settings {
	return &Settings{
		VATRatePercent:        16,
		LineDescriptionPrefix: "Продукт специализированный, для энтерального питания",
		DefaultUnit:           "штука",
		UpdatedAt:             time.Now().UTC(),
	}
}

// Validate checks the invariants of the settings document.
func (s *Settings) Validate() error {
	if s.VATRatePercent < 0 || s.VATRatePercent > 100 {
		return fmt.Errorf("ставка НДС должна быть в диапазоне 0..100")
	}
	if strings.TrimSpace(s.DefaultUnit) == "" {
		return fmt.Errorf("единица измерения обязательна")
	}
	return nil
}

// Normalize trims the free-text fields in place.
func (s *Settings) Normalize() {
	s.LineDescriptionPrefix = strings.TrimSpace(s.LineDescriptionPrefix)
	s.DefaultUnit = strings.TrimSpace(s.DefaultUnit)
}
