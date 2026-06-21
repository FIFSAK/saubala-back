package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	positionsvc "github.com/FIFSAK/saubala-back/internal/service/position"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// PositionHandler exposes the positions (warehouse batches) endpoints.
type PositionHandler struct {
	positions *positionsvc.Service
}

func NewPositionHandler(positions *positionsvc.Service) *PositionHandler {
	return &PositionHandler{positions: positions}
}

func (h *PositionHandler) Register(r chi.Router) {
	r.Get("/positions", h.List)
	r.Post("/positions", h.Create)
	r.Get("/positions/{id}", h.Get)
	r.Patch("/positions/{id}", h.Update)
	r.Delete("/positions/{id}", h.Delete)
	r.Get("/positions/{id}/movements", h.Movements)
}

type positionResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BrandID       string    `json:"brand_id"`
	ContractName  string    `json:"contract_name"`
	SupplierName  string    `json:"supplier_name"`
	ExpiryDate    time.Time `json:"expiry_date"`
	LotNumber     string    `json:"lot_number"`
	PurchasePrice int64     `json:"purchase_price"`
	Quantity      int       `json:"quantity"`
	MassGrams     int       `json:"mass_grams"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toPositionResponse(p *domain.Position) positionResponse {
	return positionResponse{
		ID:            p.ID,
		Name:          p.Name,
		BrandID:       p.BrandID,
		ContractName:  p.ContractName,
		SupplierName:  p.SupplierName,
		ExpiryDate:    p.ExpiryDate,
		LotNumber:     p.LotNumber,
		PurchasePrice: p.PurchasePrice,
		Quantity:      p.Quantity,
		MassGrams:     p.MassGrams,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

type createPositionRequest struct {
	Name          string    `json:"name"`
	BrandID       string    `json:"brand_id"`
	ContractName  string    `json:"contract_name"`
	SupplierName  string    `json:"supplier_name"`
	ExpiryDate    time.Time `json:"expiry_date"`
	LotNumber     string    `json:"lot_number"`
	PurchasePrice int64     `json:"purchase_price"`
	Quantity      int       `json:"quantity"`
	MassGrams     int       `json:"mass_grams"`
}

type updatePositionRequest struct {
	Name          *string    `json:"name"`
	BrandID       *string    `json:"brand_id"`
	ContractName  *string    `json:"contract_name"`
	SupplierName  *string    `json:"supplier_name"`
	ExpiryDate    *time.Time `json:"expiry_date"`
	LotNumber     *string    `json:"lot_number"`
	PurchasePrice *int64     `json:"purchase_price"`
	MassGrams     *int       `json:"mass_grams"`
}

func (h *PositionHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)

	before, err := queryTimePtr(r, "expiry_before")
	if err != nil {
		web.WriteError(w, err)
		return
	}
	after, err := queryTimePtr(r, "expiry_after")
	if err != nil {
		web.WriteError(w, err)
		return
	}
	inStock := queryBoolPtr(r, "in_stock")

	filter := domain.Filter{
		Q:            p.Q,
		BrandID:      r.URL.Query().Get("brand_id"),
		ExpiryBefore: before,
		ExpiryAfter:  after,
		InStock:      inStock != nil && *inStock,
		Page:         p.Page,
		PageSize:     p.PageSize,
		Sort:         p.Sort,
		Order:        p.Order,
	}

	positions, total, err := h.positions.List(r.Context(), filter)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]positionResponse, len(positions))
	for i := range positions {
		items[i] = toPositionResponse(&positions[i])
	}
	web.List(w, items, total, p)
}

func (h *PositionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPositionRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	p, err := h.positions.Create(r.Context(), positionsvc.CreateInput{
		Name:          req.Name,
		BrandID:       req.BrandID,
		ContractName:  req.ContractName,
		SupplierName:  req.SupplierName,
		ExpiryDate:    req.ExpiryDate,
		LotNumber:     req.LotNumber,
		PurchasePrice: req.PurchasePrice,
		Quantity:      req.Quantity,
		MassGrams:     req.MassGrams,
		CreatedBy:     actorID(actor),
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toPositionResponse(p))
}

func (h *PositionHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.positions.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toPositionResponse(p))
}

func (h *PositionHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updatePositionRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	p, err := h.positions.Update(r.Context(), chi.URLParam(r, "id"), positionsvc.UpdateInput{
		Name:          req.Name,
		BrandID:       req.BrandID,
		ContractName:  req.ContractName,
		SupplierName:  req.SupplierName,
		ExpiryDate:    req.ExpiryDate,
		LotNumber:     req.LotNumber,
		PurchasePrice: req.PurchasePrice,
		MassGrams:     req.MassGrams,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toPositionResponse(p))
}

func (h *PositionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.positions.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		web.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PositionHandler) Movements(w http.ResponseWriter, r *http.Request) {
	movements, err := h.positions.Movements(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	if movements == nil {
		movements = []domain.Movement{}
	}
	web.JSON(w, http.StatusOK, movements)
}
