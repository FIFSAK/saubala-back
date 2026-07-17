package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/settings"
	settingssvc "github.com/FIFSAK/saubala-back/internal/service/settings"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// SettingsHandler exposes the organization settings singleton (seller details +
// invoice defaults). Reads are open to any authenticated user (the invoice form
// prefills from them); writes require an admin (mounted under RequireAdmin).
type SettingsHandler struct {
	settings *settingssvc.Service
}

func NewSettingsHandler(settings *settingssvc.Service) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

// RegisterRead mounts the read-only settings route (any authenticated user).
func (h *SettingsHandler) RegisterRead(r chi.Router) {
	r.Get("/settings", h.Get)
}

// RegisterWrite mounts the settings mutation route (admin only).
func (h *SettingsHandler) RegisterWrite(r chi.Router) {
	r.Put("/settings", h.Update)
}

type settingsResponse struct {
	OrgName               string    `json:"org_name"`
	BIN                   string    `json:"bin"`
	ResponsibleForSupply  string    `json:"responsible_for_supply"`
	Director              string    `json:"director"`
	Accountant            string    `json:"accountant"`
	VATRatePercent        int       `json:"vat_rate_percent"`
	LineDescriptionPrefix string    `json:"line_description_prefix"`
	DefaultUnit           string    `json:"default_unit"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func toSettingsResponse(o *domain.Organization) settingsResponse {
	return settingsResponse{
		OrgName:               o.OrgName,
		BIN:                   o.BIN,
		ResponsibleForSupply:  o.ResponsibleForSupply,
		Director:              o.Director,
		Accountant:            o.Accountant,
		VATRatePercent:        o.VATRatePercent,
		LineDescriptionPrefix: o.LineDescriptionPrefix,
		DefaultUnit:           o.DefaultUnit,
		UpdatedAt:             o.UpdatedAt,
	}
}

type updateSettingsRequest struct {
	OrgName               string `json:"org_name"`
	BIN                   string `json:"bin"`
	ResponsibleForSupply  string `json:"responsible_for_supply"`
	Director              string `json:"director"`
	Accountant            string `json:"accountant"`
	VATRatePercent        int    `json:"vat_rate_percent"`
	LineDescriptionPrefix string `json:"line_description_prefix"`
	DefaultUnit           string `json:"default_unit"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	o, err := h.settings.Get(r.Context())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSettingsResponse(o))
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	o := &domain.Organization{
		OrgName:               req.OrgName,
		BIN:                   req.BIN,
		ResponsibleForSupply:  req.ResponsibleForSupply,
		Director:              req.Director,
		Accountant:            req.Accountant,
		VATRatePercent:        req.VATRatePercent,
		LineDescriptionPrefix: req.LineDescriptionPrefix,
		DefaultUnit:           req.DefaultUnit,
	}
	updated, err := h.settings.Update(r.Context(), o)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSettingsResponse(updated))
}
