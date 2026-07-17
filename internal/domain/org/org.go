// Package org holds the sender organizations (наши фирмы-отправители, e.g.
// ТОО «Смак-МН» and ИП «Онко»). One of them is chosen on every release and its
// details are printed as the seller side of the waybill.
package org

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var binRe = regexp.MustCompile(`^\d{12}$`)

// Organization is a sender (seller) organization used on release waybills.
type Organization struct {
	ID                   string
	Name                 string // «Организация» / отправитель, e.g. ТОО «Смак-МН»
	BIN                  string // ИИН/БИН of the seller, 12 digits
	ResponsibleForSupply string // «Ответственный за поставку (Ф.И.О.)»
	Director             string // «Отпуск разрешил» — руководитель, Ф.И.О.
	Accountant           string // «Главный бухгалтер» — Ф.И.О. (или «Не предусмотрен»)
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// New constructs a validated organization.
func New(name, bin, responsibleForSupply, director, accountant string) (*Organization, error) {
	now := time.Now().UTC()
	o := &Organization{
		ID:                   uuid.NewString(),
		Name:                 name,
		BIN:                  bin,
		ResponsibleForSupply: responsibleForSupply,
		Director:             director,
		Accountant:           accountant,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	o.Normalize()
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return o, nil
}

// Default returns the organization seeded on first startup, pre-filled with the
// customer's current values.
func Default() *Organization {
	now := time.Now().UTC()
	return &Organization{
		ID:                   uuid.NewString(),
		Name:                 "ТОО «Смак-МН»",
		BIN:                  "201140002658",
		ResponsibleForSupply: "Руководитель Турсынбекова Айгерим Аманжоловна",
		Director:             "Руководитель Турсынбекова Айгерим",
		Accountant:           "Не предусмотрен",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// Validate checks the invariants of the organization.
func (o *Organization) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("наименование организации обязательно")
	}
	if !binRe.MatchString(strings.TrimSpace(o.BIN)) {
		return fmt.Errorf("БИН должен состоять ровно из 12 цифр")
	}
	return nil
}

// Normalize trims the free-text fields in place.
func (o *Organization) Normalize() {
	o.Name = strings.TrimSpace(o.Name)
	o.BIN = strings.TrimSpace(o.BIN)
	o.ResponsibleForSupply = strings.TrimSpace(o.ResponsibleForSupply)
	o.Director = strings.TrimSpace(o.Director)
	o.Accountant = strings.TrimSpace(o.Accountant)
}
