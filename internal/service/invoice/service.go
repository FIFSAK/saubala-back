// Package invoice implements the use case of generating a release waybill
// (Форма З-2) for an existing release, in XLSX or PDF. Generation is stateless:
// nothing is persisted; the manual header fields are supplied per request.
package invoice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	settingsdom "github.com/FIFSAK/saubala-back/internal/domain/settings"
	"github.com/FIFSAK/saubala-back/internal/invoice"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service generates release waybills from stored releases plus per-request
// manual header fields and the organization settings.
type Service struct {
	releases  release.Repository
	contracts contract.Repository
	positions position.Repository
	settings  settingsdom.Repository
}

func NewService(releases release.Repository, contracts contract.Repository, positions position.Repository, settings settingsdom.Repository) *Service {
	return &Service{releases: releases, contracts: contracts, positions: positions, settings: settings}
}

// Format is the output document format.
type Format string

const (
	FormatXLSX Format = "xlsx"
	FormatPDF  Format = "pdf"
)

// GenerateInput carries the release id and the four manually-entered header
// fields (document number, date, recipient name and address).
type GenerateInput struct {
	ReleaseID        string
	Format           Format
	DocumentNumber   string
	DocumentDate     time.Time
	RecipientName    string
	RecipientAddress string
}

// GenerateOutput is the rendered file plus its download metadata.
type GenerateOutput struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Generate builds and renders the waybill for the given release.
func (s *Service) Generate(ctx context.Context, in GenerateInput) (*GenerateOutput, error) {
	if err := validate(in); err != nil {
		return nil, err
	}

	rel, err := s.releases.GetByID(ctx, in.ReleaseID)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("отгрузка не найдена")
		}
		return nil, err
	}

	// Contract lines carry the planned (sale) price per line; missing contract is
	// tolerated — prices then fall back to the position/release cost.
	plannedPrice := make(map[string]*int64)
	if c, err := s.contracts.GetByID(ctx, rel.ContractID); err == nil {
		for _, cl := range c.Lines {
			plannedPrice[cl.ID] = cl.PlannedPrice
		}
	} else if !errors.Is(err, store.ErrorNotFound) {
		return nil, err
	}

	posByID, err := s.positionsByID(ctx, rel.Lines)
	if err != nil {
		return nil, err
	}

	set, err := s.currentSettings(ctx)
	if err != nil {
		return nil, err
	}

	lines := make([]invoice.LineInput, len(rel.Lines))
	for i, l := range rel.Lines {
		pos := posByID[l.PositionID]
		lines[i] = invoice.LineInput{
			Name:      lineName(set.LineDescriptionPrefix, pos, l.PositionID),
			Unit:      set.DefaultUnit,
			Quantity:  l.Quantity,
			UnitPrice: unitPrice(plannedPrice[l.ContractLineID], pos, l.UnitCost),
		}
	}

	inv := invoice.Build(invoice.BuildInput{
		DocumentNumber:       in.DocumentNumber,
		DocumentDate:         in.DocumentDate,
		SellerName:           set.OrgName,
		SellerBIN:            set.BIN,
		ResponsibleForSupply: set.ResponsibleForSupply,
		Director:             set.Director,
		Accountant:           set.Accountant,
		RecipientName:        in.RecipientName,
		RecipientAddress:     in.RecipientAddress,
		VATRatePercent:       set.VATRatePercent,
		Lines:                lines,
	})

	return render(inv, in.Format)
}

func validate(in GenerateInput) error {
	if in.Format != FormatXLSX && in.Format != FormatPDF {
		return web.BadRequest("формат должен быть «xlsx» или «pdf»")
	}
	if strings.TrimSpace(in.DocumentNumber) == "" {
		return web.BadRequest("номер документа обязателен")
	}
	if in.DocumentDate.IsZero() {
		return web.BadRequest("дата документа обязательна")
	}
	if strings.TrimSpace(in.RecipientName) == "" {
		return web.BadRequest("наименование получателя обязательно")
	}
	if strings.TrimSpace(in.RecipientAddress) == "" {
		return web.BadRequest("адрес получателя обязателен")
	}
	return nil
}

// positionsByID batch-loads the positions referenced by the release lines.
func (s *Service) positionsByID(ctx context.Context, lines []release.Line) (map[string]*position.Position, error) {
	seen := make(map[string]struct{})
	var ids []string
	for _, l := range lines {
		if _, ok := seen[l.PositionID]; ok {
			continue
		}
		seen[l.PositionID] = struct{}{}
		ids = append(ids, l.PositionID)
	}
	positions, err := s.positions.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*position.Position, len(positions))
	for i := range positions {
		out[positions[i].ID] = &positions[i]
	}
	return out, nil
}

// currentSettings returns the stored settings, falling back to the seeded
// defaults when the singleton is somehow absent.
func (s *Service) currentSettings(ctx context.Context) (*settingsdom.Organization, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return settingsdom.Default(), nil
		}
		return nil, err
	}
	return set, nil
}

// unitPrice picks the invoice unit price: the contract line's planned price when
// present, else the position purchase price, else the release-line unit cost.
func unitPrice(planned *int64, pos *position.Position, releaseCost int64) int64 {
	if planned != nil {
		return *planned
	}
	if pos != nil {
		return pos.PurchasePrice
	}
	return releaseCost
}

// lineName renders the «наименование, характеристика» text, wrapping the position
// name with the configured prefix and its expiry date.
func lineName(prefix string, pos *position.Position, fallbackID string) string {
	if pos == nil {
		return fallbackID
	}
	expiry := pos.ExpiryDate.Format("02.01.2006")
	if strings.TrimSpace(prefix) == "" {
		return fmt.Sprintf("%s (Срок годности: %s)", pos.Name, expiry)
	}
	return fmt.Sprintf("%s(%s, Срок годности: %s)", prefix, pos.Name, expiry)
}

// render dispatches to the format-specific renderer and attaches download
// metadata (content type + Cyrillic filename).
func render(inv *invoice.Invoice, format Format) (*GenerateOutput, error) {
	switch format {
	case FormatXLSX:
		data, err := invoice.RenderXLSX(inv)
		if err != nil {
			return nil, err
		}
		return &GenerateOutput{
			Filename:    filename(inv, "xlsx"),
			ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			Data:        data,
		}, nil
	case FormatPDF:
		data, err := invoice.RenderPDF(inv)
		if err != nil {
			return nil, err
		}
		return &GenerateOutput{
			Filename:    filename(inv, "pdf"),
			ContentType: "application/pdf",
			Data:        data,
		}, nil
	default:
		return nil, web.BadRequest("формат должен быть «xlsx» или «pdf»")
	}
}

func filename(inv *invoice.Invoice, ext string) string {
	return fmt.Sprintf("Накладная №%s от %s.%s", inv.DocumentNumber, inv.DocumentDate.Format("02.01.2006"), ext)
}
