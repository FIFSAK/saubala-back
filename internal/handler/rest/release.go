package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	releasesvc "github.com/FIFSAK/saubala-back/internal/service/release"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// ReleaseHandler exposes the releases (outbound stock against a contract) endpoints.
type ReleaseHandler struct {
	releases *releasesvc.Service
}

func NewReleaseHandler(releases *releasesvc.Service) *ReleaseHandler {
	return &ReleaseHandler{releases: releases}
}

func (h *ReleaseHandler) Register(r chi.Router) {
	r.Get("/releases", h.List)
	r.Post("/releases", h.Create)
	r.Get("/releases/{id}", h.Get)
}

type releaseLineDTO struct {
	ContractLineID string `json:"contract_line_id"`
	PositionID     string `json:"position_id"`
	PositionName   string `json:"position_name"`
	LotNumber      string `json:"lot_number"`
	Quantity       int    `json:"quantity"`
	UnitCost       int64  `json:"unit_cost"`
}

type releaseResponse struct {
	ID             string           `json:"id"`
	ContractID     string           `json:"contract_id"`
	ContractNumber string           `json:"contract_number"`
	ContractName   string           `json:"contract_name"`
	Date           time.Time        `json:"date"`
	Note           string           `json:"note"`
	Lines          []releaseLineDTO `json:"lines"`
	CreatedBy      string           `json:"created_by"`
	CreatedAt      time.Time        `json:"created_at"`
}

func toReleaseResponse(rel *domain.Release, crefs map[string]releasesvc.ContractRef, prefs map[string]releasesvc.PositionRef) releaseResponse {
	lines := make([]releaseLineDTO, len(rel.Lines))
	for i, l := range rel.Lines {
		pref := prefs[l.PositionID]
		lines[i] = releaseLineDTO{
			ContractLineID: l.ContractLineID,
			PositionID:     l.PositionID,
			PositionName:   pref.Name,
			LotNumber:      pref.LotNumber,
			Quantity:       l.Quantity,
			UnitCost:       l.UnitCost,
		}
	}
	cref := crefs[rel.ContractID]
	return releaseResponse{
		ID:             rel.ID,
		ContractID:     rel.ContractID,
		ContractNumber: cref.Number,
		ContractName:   cref.Name,
		Date:           rel.Date,
		Note:           rel.Note,
		Lines:          lines,
		CreatedBy:      rel.CreatedBy,
		CreatedAt:      rel.CreatedAt,
	}
}

type releaseLineRequest struct {
	ContractLineID string `json:"contract_line_id"`
	PositionID     string `json:"position_id"`
	Quantity       int    `json:"quantity"`
}

type createReleaseRequest struct {
	ContractID string               `json:"contract_id"`
	Date       time.Time            `json:"date"`
	Note       string               `json:"note"`
	Lines      []releaseLineRequest `json:"lines"`
}

func (h *ReleaseHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)
	from, err := queryTimePtr(r, "date_from")
	if err != nil {
		web.WriteError(w, err)
		return
	}
	to, err := queryTimePtr(r, "date_to")
	if err != nil {
		web.WriteError(w, err)
		return
	}

	releases, total, err := h.releases.List(r.Context(), domain.Filter{
		ContractID: r.URL.Query().Get("contract_id"),
		DateFrom:   from,
		DateTo:     to,
		Page:       p.Page,
		PageSize:   p.PageSize,
		Sort:       p.Sort,
		Order:      p.Order,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	crefs, prefs, err := h.releases.Refs(r.Context(), releases)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]releaseResponse, len(releases))
	for i := range releases {
		items[i] = toReleaseResponse(&releases[i], crefs, prefs)
	}
	web.List(w, items, total, p)
}

func (h *ReleaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReleaseRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	lines := make([]releasesvc.LineInput, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = releasesvc.LineInput{
			ContractLineID: l.ContractLineID,
			PositionID:     l.PositionID,
			Quantity:       l.Quantity,
		}
	}

	rel, err := h.releases.Create(r.Context(), releasesvc.CreateInput{
		ContractID: req.ContractID,
		Date:       req.Date,
		Note:       req.Note,
		Lines:      lines,
		CreatedBy:  actorID(actor),
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	crefs, prefs, err := h.releases.Refs(r.Context(), []domain.Release{*rel})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toReleaseResponse(rel, crefs, prefs))
}

func (h *ReleaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	rel, err := h.releases.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	crefs, prefs, err := h.releases.Refs(r.Context(), []domain.Release{*rel})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toReleaseResponse(rel, crefs, prefs))
}
