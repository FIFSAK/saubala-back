package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/org"
	orgsvc "github.com/FIFSAK/saubala-back/internal/service/org"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// OrgHandler exposes the sender organizations. Reads are open to any
// authenticated user (the release form picks the sender from them); mutations
// require an admin (mounted under RequireAdmin).
type OrgHandler struct {
	orgs *orgsvc.Service
}

func NewOrgHandler(orgs *orgsvc.Service) *OrgHandler {
	return &OrgHandler{orgs: orgs}
}

// RegisterRead mounts the read-only organization routes (any authenticated user).
func (h *OrgHandler) RegisterRead(r chi.Router) {
	r.Get("/organizations", h.List)
}

// RegisterWrite mounts the organization mutation routes (admin only).
func (h *OrgHandler) RegisterWrite(r chi.Router) {
	r.Post("/organizations", h.Create)
	r.Put("/organizations/{id}", h.Update)
	r.Delete("/organizations/{id}", h.Delete)
}

type orgResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	BIN                  string    `json:"bin"`
	ResponsibleForSupply string    `json:"responsible_for_supply"`
	Director             string    `json:"director"`
	Accountant           string    `json:"accountant"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func toOrgResponse(o *domain.Organization) orgResponse {
	return orgResponse{
		ID:                   o.ID,
		Name:                 o.Name,
		BIN:                  o.BIN,
		ResponsibleForSupply: o.ResponsibleForSupply,
		Director:             o.Director,
		Accountant:           o.Accountant,
		CreatedAt:            o.CreatedAt,
		UpdatedAt:            o.UpdatedAt,
	}
}

type orgRequest struct {
	Name                 string `json:"name"`
	BIN                  string `json:"bin"`
	ResponsibleForSupply string `json:"responsible_for_supply"`
	Director             string `json:"director"`
	Accountant           string `json:"accountant"`
}

func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.orgs.List(r.Context())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]orgResponse, len(orgs))
	for i := range orgs {
		items[i] = toOrgResponse(&orgs[i])
	}
	web.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req orgRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	o, err := h.orgs.Create(r.Context(), orgsvc.Input{
		Name:                 req.Name,
		BIN:                  req.BIN,
		ResponsibleForSupply: req.ResponsibleForSupply,
		Director:             req.Director,
		Accountant:           req.Accountant,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toOrgResponse(o))
}

func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req orgRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	o, err := h.orgs.Update(r.Context(), chi.URLParam(r, "id"), orgsvc.Input{
		Name:                 req.Name,
		BIN:                  req.BIN,
		ResponsibleForSupply: req.ResponsibleForSupply,
		Director:             req.Director,
		Accountant:           req.Accountant,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toOrgResponse(o))
}

func (h *OrgHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.orgs.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		web.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
