// Package invoice implements the use case of generating a release waybill
// (Форма З-2) for an existing release, in XLSX or PDF. Generation is stateless:
// nothing is persisted. The header data (document number, recipient, sender
// organization) is taken from the release itself — it is captured at release
// time — while the request fields remain as optional overrides.
package invoice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	orgdom "github.com/FIFSAK/saubala-back/internal/domain/org"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	settingsdom "github.com/FIFSAK/saubala-back/internal/domain/settings"
	"github.com/FIFSAK/saubala-back/internal/invoice"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service generates release waybills from stored releases, the sender
// organization chosen at release time and the invoice defaults.
type Service struct {
	releases  release.Repository
	contracts contract.Repository
	positions position.Repository
	orgs      orgdom.Repository
	settings  settingsdom.Repository
}

func NewService(releases release.Repository, contracts contract.Repository, positions position.Repository, orgs orgdom.Repository, settings settingsdom.Repository) *Service {
	return &Service{releases: releases, contracts: contracts, positions: positions, orgs: orgs, settings: settings}
}

// Format is the output document format.
type Format string

const (
	FormatXLSX Format = "xlsx"
	FormatPDF  Format = "pdf"
)

// GenerateInput carries the release id, the output format, and optional header
// overrides. Empty override fields fall back to the data stored on the release
// (and, for the recipient, to the contract).
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
	if in.Format != FormatXLSX && in.Format != FormatPDF {
		return nil, web.BadRequest("формат должен быть «xlsx» или «pdf»")
	}

	rel, err := s.releases.GetByID(ctx, in.ReleaseID)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("отгрузка не найдена")
		}
		return nil, err
	}

	// Contract lines carry the planned (sale) price and the per-contract product
	// name; a missing contract is tolerated — prices then fall back to the
	// position/release cost and names to the position.
	plannedPrice := make(map[string]*int64)
	contractName := make(map[string]string)
	var c *contract.Contract
	if rel.ContractID != "" {
		c, err = s.contracts.GetByID(ctx, rel.ContractID)
		if err == nil {
			for _, cl := range c.Lines {
				plannedPrice[cl.ID] = cl.PlannedPrice
				contractName[cl.ID] = cl.ContractName
			}
		} else if !errors.Is(err, store.ErrorNotFound) {
			return nil, err
		}
	}

	header, err := resolveHeader(in, rel, c)
	if err != nil {
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
	seller, err := s.seller(ctx, rel.OrganizationID)
	if err != nil {
		return nil, err
	}

	lines := make([]invoice.LineInput, len(rel.Lines))
	for i, l := range rel.Lines {
		pos := posByID[l.PositionID]
		lines[i] = invoice.LineInput{
			Name:      lineName(set.LineDescriptionPrefix, contractName[l.ContractLineID], pos, l.PositionID),
			Unit:      set.DefaultUnit,
			Quantity:  l.Quantity,
			UnitPrice: unitPrice(l, plannedPrice[l.ContractLineID], pos),
		}
	}

	inv := invoice.Build(invoice.BuildInput{
		DocumentNumber:       header.number,
		DocumentDate:         header.date,
		SellerName:           seller.Name,
		SellerBIN:            seller.BIN,
		ResponsibleForSupply: seller.ResponsibleForSupply,
		Director:             seller.Director,
		Accountant:           seller.Accountant,
		RecipientName:        header.recipientName,
		RecipientAddress:     header.recipientAddress,
		VATRatePercent:       set.VATRatePercent,
		Lines:                lines,
	})

	return render(inv, in.Format)
}

type header struct {
	number           string
	date             time.Time
	recipientName    string
	recipientAddress string
}

// resolveHeader merges the request overrides with the data stored on the
// release, falling back to the contract for the recipient.
func resolveHeader(in GenerateInput, rel *release.Release, c *contract.Contract) (*header, error) {
	h := &header{
		number:           strings.TrimSpace(in.DocumentNumber),
		date:             in.DocumentDate,
		recipientName:    strings.TrimSpace(in.RecipientName),
		recipientAddress: strings.TrimSpace(in.RecipientAddress),
	}
	if h.number == "" {
		h.number = rel.DocumentNumber
	}
	if h.date.IsZero() {
		h.date = rel.Date
	}
	if h.recipientName == "" {
		h.recipientName = rel.RecipientName
	}
	if h.recipientAddress == "" {
		h.recipientAddress = rel.RecipientAddress
	}
	if c != nil {
		if h.recipientName == "" {
			// The official customer name wins over the short working caption.
			h.recipientName = c.CustomerOfficialName
			if h.recipientName == "" {
				h.recipientName = c.Name
			}
		}
		if h.recipientAddress == "" {
			h.recipientAddress = c.CustomerAddress
		}
	}
	if h.number == "" {
		return nil, web.BadRequest("номер накладной не указан — заполните его в отгрузке или передайте в запросе")
	}
	if h.recipientName == "" {
		return nil, web.BadRequest("наименование получателя обязательно")
	}
	if h.recipientAddress == "" {
		return nil, web.BadRequest("адрес получателя обязателен")
	}
	return h, nil
}

// seller resolves the sender organization chosen at release time, falling back
// to the first configured organization (and, if none exist, to the seeded
// default) for releases persisted before the choice existed.
func (s *Service) seller(ctx context.Context, organizationID string) (*orgdom.Organization, error) {
	if organizationID != "" {
		o, err := s.orgs.GetByID(ctx, organizationID)
		if err == nil {
			return o, nil
		}
		if !errors.Is(err, store.ErrorNotFound) {
			return nil, err
		}
	}
	list, err := s.orgs.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(list) > 0 {
		return &list[0], nil
	}
	return orgdom.Default(), nil
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
func (s *Service) currentSettings(ctx context.Context) (*settingsdom.Settings, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return settingsdom.Default(), nil
		}
		return nil, err
	}
	return set, nil
}

// unitPrice picks the invoice unit price: the sale price stored at release time
// when present, else — for contract releases persisted before prices were
// stored — the contract line's planned price, the position purchase price, or
// the release-line unit cost. Free-release lines stay at 0.
func unitPrice(l release.Line, planned *int64, pos *position.Position) int64 {
	if l.UnitPrice > 0 {
		return l.UnitPrice
	}
	if l.ContractLineID == "" {
		return 0
	}
	if planned != nil {
		return *planned
	}
	if pos != nil {
		return pos.PurchasePrice
	}
	return l.UnitCost
}

// lineName renders the «наименование, характеристика» text, wrapping the product
// name with the configured prefix and the batch expiry date. The name written in
// the contract wins over the position's own contract name, then its plain name.
func lineName(prefix, contractLineName string, pos *position.Position, fallbackID string) string {
	name := strings.TrimSpace(contractLineName)
	expiry := ""
	if pos != nil {
		if name == "" {
			name = strings.TrimSpace(pos.ContractName)
		}
		if name == "" {
			name = pos.Name
		}
		expiry = pos.ExpiryDate.Format("02.01.2006")
	}
	if name == "" {
		return fallbackID
	}
	if expiry == "" {
		if strings.TrimSpace(prefix) == "" {
			return name
		}
		return fmt.Sprintf("%s(%s)", prefix, name)
	}
	if strings.TrimSpace(prefix) == "" {
		return fmt.Sprintf("%s (Срок годности: %s)", name, expiry)
	}
	return fmt.Sprintf("%s(%s, Срок годности: %s)", prefix, name, expiry)
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
