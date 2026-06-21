package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/brand"
	brandsvc "github.com/FIFSAK/saubala-back/internal/service/brand"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// BrandHandler exposes the brand catalogue endpoints.
type BrandHandler struct {
	brands *brandsvc.Service
}

func NewBrandHandler(brands *brandsvc.Service) *BrandHandler {
	return &BrandHandler{brands: brands}
}

func (h *BrandHandler) Register(r chi.Router) {
	r.Get("/brands", h.List)
	r.Post("/brands", h.Create)
	r.Get("/brands/{id}", h.Get)
	r.Patch("/brands/{id}", h.Update)
	r.Delete("/brands/{id}", h.Delete)
}

type brandResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toBrandResponse(b *domain.Brand) brandResponse {
	return brandResponse{
		ID:        b.ID,
		Name:      b.Name,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

type brandRequest struct {
	Name string `json:"name"`
}

func (h *BrandHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)
	brands, total, err := h.brands.List(r.Context(), domain.Filter{
		Q:        p.Q,
		Page:     p.Page,
		PageSize: p.PageSize,
		Sort:     p.Sort,
		Order:    p.Order,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]brandResponse, len(brands))
	for i := range brands {
		items[i] = toBrandResponse(&brands[i])
	}
	web.List(w, items, total, p)
}

func (h *BrandHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req brandRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	b, err := h.brands.Create(r.Context(), req.Name)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toBrandResponse(b))
}

func (h *BrandHandler) Get(w http.ResponseWriter, r *http.Request) {
	b, err := h.brands.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toBrandResponse(b))
}

func (h *BrandHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req brandRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	b, err := h.brands.Update(r.Context(), chi.URLParam(r, "id"), req.Name)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toBrandResponse(b))
}

func (h *BrandHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.brands.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		web.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
