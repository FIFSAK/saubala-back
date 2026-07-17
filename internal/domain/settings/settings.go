// Package settings holds the organization-wide settings singleton: the seller
// details and invoice defaults used when generating release waybills (Форма З-2).
package settings

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ID is the fixed identifier of the singleton settings document.
const ID = "org"

var binRe = regexp.MustCompile(`^\d{12}$`)

// Organization is the singleton settings document. It carries the seller-side
// values printed on every release waybill (our own company), plus the invoice
// defaults (VAT rate, line-description prefix, unit of measure).
//
// It is not user-scoped: exactly one document exists, seeded on startup and
// edited from the settings page.
type Organization struct {
	OrgName               string // «Организация» / отправитель, e.g. ТОО «Смак-МН»
	BIN                   string // ИИН/БИН of the seller, 12 digits
	ResponsibleForSupply  string // «Ответственный за поставку (Ф.И.О.)»
	Director              string // «Отпуск разрешил» — руководитель, Ф.И.О.
	Accountant            string // «Главный бухгалтер» — Ф.И.О. (или «Не предусмотрен»)
	VATRatePercent        int    // НДС, %, prices are treated as VAT-inclusive
	LineDescriptionPrefix string // prefix wrapping each position name in column «Наименование»
	DefaultUnit           string // «Единица измерения», e.g. штука
	UpdatedAt             time.Time
}

// Default returns the settings seeded on first startup, pre-filled with the
// customer's current values.
func Default() *Organization {
	return &Organization{
		OrgName:               "ТОО «Смак-МН»",
		BIN:                   "201140002658",
		ResponsibleForSupply:  "Руководитель Турсынбекова Айгерим Аманжоловна",
		Director:              "Руководитель Турсынбекова Айгерим",
		Accountant:            "Не предусмотрен",
		VATRatePercent:        16,
		LineDescriptionPrefix: "Продукт специализированный, для энтерального питания",
		DefaultUnit:           "штука",
		UpdatedAt:             time.Now().UTC(),
	}
}

// Validate checks the invariants of the settings document.
func (o *Organization) Validate() error {
	if strings.TrimSpace(o.OrgName) == "" {
		return fmt.Errorf("наименование организации обязательно")
	}
	if !binRe.MatchString(strings.TrimSpace(o.BIN)) {
		return fmt.Errorf("БИН должен состоять ровно из 12 цифр")
	}
	if o.VATRatePercent < 0 || o.VATRatePercent > 100 {
		return fmt.Errorf("ставка НДС должна быть в диапазоне 0..100")
	}
	if strings.TrimSpace(o.DefaultUnit) == "" {
		return fmt.Errorf("единица измерения обязательна")
	}
	return nil
}

// Normalize trims the free-text fields in place.
func (o *Organization) Normalize() {
	o.OrgName = strings.TrimSpace(o.OrgName)
	o.BIN = strings.TrimSpace(o.BIN)
	o.ResponsibleForSupply = strings.TrimSpace(o.ResponsibleForSupply)
	o.Director = strings.TrimSpace(o.Director)
	o.Accountant = strings.TrimSpace(o.Accountant)
	o.LineDescriptionPrefix = strings.TrimSpace(o.LineDescriptionPrefix)
	o.DefaultUnit = strings.TrimSpace(o.DefaultUnit)
}
