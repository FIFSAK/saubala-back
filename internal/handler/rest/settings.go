package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/settings"
	settingssvc "github.com/FIFSAK/saubala-back/internal/service/settings"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// SettingsHandler exposes the settings singleton (invoice defaults). Reads are
// open to any authenticated user (the invoice form prefills from them); writes
// require an admin (mounted under RequireAdmin).
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
	VATRatePercent        int       `json:"vat_rate_percent"`
	LineDescriptionPrefix string    `json:"line_description_prefix"`
	DefaultUnit           string    `json:"default_unit"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func toSettingsResponse(s *domain.Settings) settingsResponse {
	return settingsResponse{
		VATRatePercent:        s.VATRatePercent,
		LineDescriptionPrefix: s.LineDescriptionPrefix,
		DefaultUnit:           s.DefaultUnit,
		UpdatedAt:             s.UpdatedAt,
	}
}

type updateSettingsRequest struct {
	VATRatePercent        int    `json:"vat_rate_percent"`
	LineDescriptionPrefix string `json:"line_description_prefix"`
	DefaultUnit           string `json:"default_unit"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.settings.Get(r.Context())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSettingsResponse(s))
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	s := &domain.Settings{
		VATRatePercent:        req.VATRatePercent,
		LineDescriptionPrefix: req.LineDescriptionPrefix,
		DefaultUnit:           req.DefaultUnit,
	}
	updated, err := h.settings.Update(r.Context(), s)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSettingsResponse(updated))
}
