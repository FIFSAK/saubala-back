package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/supplier"
	suppliersvc "github.com/FIFSAK/saubala-back/internal/service/supplier"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// SupplierHandler exposes the suppliers reference endpoints.
type SupplierHandler struct {
	suppliers *suppliersvc.Service
}

func NewSupplierHandler(suppliers *suppliersvc.Service) *SupplierHandler {
	return &SupplierHandler{suppliers: suppliers}
}

func (h *SupplierHandler) Register(r chi.Router) {
	r.Get("/suppliers", h.List)
	r.Post("/suppliers", h.Create)
	r.Get("/suppliers/{id}", h.Get)
	r.Put("/suppliers/{id}", h.Update)
	r.Delete("/suppliers/{id}", h.Delete)
}

type supplierResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	BIN       string    `json:"bin"`
	Country   string    `json:"country"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// supplierListItem additionally carries the summed receipt invoice amounts —
// «на какую сумму закуплено» у поставщика.
type supplierListItem struct {
	supplierResponse
	InvoiceTotal int64 `json:"invoice_total"`
}

func toSupplierResponse(s *domain.Supplier) supplierResponse {
	return supplierResponse{
		ID:        s.ID,
		Name:      s.Name,
		Type:      string(s.Type),
		BIN:       s.BIN,
		Country:   s.Country,
		Phone:     s.Phone,
		Email:     s.Email,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

type supplierRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	BIN     string `json:"bin"`
	Country string `json:"country"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
}

func (r supplierRequest) toInput() suppliersvc.Input {
	return suppliersvc.Input{
		Name:    r.Name,
		Type:    domain.Type(r.Type),
		BIN:     r.BIN,
		Country: r.Country,
		Phone:   r.Phone,
		Email:   r.Email,
	}
}

func (h *SupplierHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)
	suppliers, total, err := h.suppliers.List(r.Context(), domain.Filter{
		Q:        p.Q,
		Type:     domain.Type(r.URL.Query().Get("type")),
		Page:     p.Page,
		PageSize: p.PageSize,
		Sort:     p.Sort,
		Order:    p.Order,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	totals, err := h.suppliers.InvoiceTotals(r.Context(), suppliers)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]supplierListItem, len(suppliers))
	for i := range suppliers {
		items[i] = supplierListItem{
			supplierResponse: toSupplierResponse(&suppliers[i]),
			InvoiceTotal:     totals[suppliers[i].ID],
		}
	}
	web.List(w, items, total, p)
}

func (h *SupplierHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.suppliers.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSupplierResponse(s))
}

func (h *SupplierHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req supplierRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	s, err := h.suppliers.Create(r.Context(), req.toInput())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toSupplierResponse(s))
}

func (h *SupplierHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req supplierRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	s, err := h.suppliers.Update(r.Context(), chi.URLParam(r, "id"), req.toInput())
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toSupplierResponse(s))
}

func (h *SupplierHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.suppliers.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		web.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
