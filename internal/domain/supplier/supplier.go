// Package supplier holds the suppliers reference (справочник поставщиков).
// A supplier is either Kazakhstani (identified by a 12-digit BIN) or foreign
// (identified by its country); positions and receipts reference suppliers so
// purchases can be attributed to them.
package supplier

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var binRe = regexp.MustCompile(`^\d{12}$`)

// Type discriminates Kazakhstani and foreign suppliers.
type Type string

const (
	TypeKZ      Type = "kz"      // казахстанский: имя + БИН
	TypeForeign Type = "foreign" // иностранный: имя + страна
)

// Supplier is one entry of the suppliers reference.
type Supplier struct {
	ID        string
	Name      string
	Type      Type
	BIN       string // 12 digits; kz only
	Country   string // foreign only
	Phone     string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New constructs a validated supplier.
func New(name string, t Type, bin, country, phone, email string) (*Supplier, error) {
	now := time.Now().UTC()
	s := &Supplier{
		ID:        uuid.NewString(),
		Name:      name,
		Type:      t,
		BIN:       bin,
		Country:   country,
		Phone:     phone,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.Normalize()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate checks the invariants of a supplier.
func (s *Supplier) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("наименование поставщика обязательно")
	}
	switch s.Type {
	case TypeKZ:
		if !binRe.MatchString(s.BIN) {
			return fmt.Errorf("БИН должен состоять ровно из 12 цифр")
		}
		if s.Country != "" {
			return fmt.Errorf("страна указывается только для иностранного поставщика")
		}
	case TypeForeign:
		if s.Country == "" {
			return fmt.Errorf("страна обязательна для иностранного поставщика")
		}
		if s.BIN != "" {
			return fmt.Errorf("БИН указывается только для казахстанского поставщика")
		}
	default:
		return fmt.Errorf("тип поставщика должен быть «kz» или «foreign»")
	}
	if s.Email != "" {
		if _, err := mail.ParseAddress(s.Email); err != nil {
			return fmt.Errorf("некорректный e-mail")
		}
	}
	return nil
}

// Normalize trims the free-text fields in place.
func (s *Supplier) Normalize() {
	s.Name = strings.TrimSpace(s.Name)
	s.BIN = strings.TrimSpace(s.BIN)
	s.Country = strings.TrimSpace(s.Country)
	s.Phone = strings.TrimSpace(s.Phone)
	s.Email = strings.TrimSpace(s.Email)
}
