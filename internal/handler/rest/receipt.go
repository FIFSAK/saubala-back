package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	receiptsvc "github.com/FIFSAK/saubala-back/internal/service/receipt"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// ReceiptHandler exposes the receipts (inbound stock) endpoints.
type ReceiptHandler struct {
	receipts *receiptsvc.Service
}

func NewReceiptHandler(receipts *receiptsvc.Service) *ReceiptHandler {
	return &ReceiptHandler{receipts: receipts}
}

func (h *ReceiptHandler) Register(r chi.Router) {
	r.Get("/receipts", h.List)
	r.Post("/receipts", h.Create)
	r.Get("/receipts/{id}", h.Get)
}

type receiptLineDTO struct {
	PositionID   string `json:"position_id"`
	PositionName string `json:"position_name"`
	LotNumber    string `json:"lot_number"`
	BrandName    string `json:"brand_name"`
	Quantity     int    `json:"quantity"`
}

type receiptResponse struct {
	ID            string           `json:"id"`
	Date          time.Time        `json:"date"`
	Note          string           `json:"note"`
	SupplierID    string           `json:"supplier_id"`
	SupplierName  string           `json:"supplier_name"`
	Counterparty  string           `json:"counterparty"` // legacy free-text supplier of old receipts
	InvoiceAmount int64            `json:"invoice_amount"`
	Lines         []receiptLineDTO `json:"lines"`
	CreatedBy     string           `json:"created_by"`
	CreatedAt     time.Time        `json:"created_at"`
}

func toReceiptResponse(rec *domain.Receipt, prefs map[string]receiptsvc.PositionRef, supplierNames map[string]string) receiptResponse {
	lines := make([]receiptLineDTO, len(rec.Lines))
	for i, l := range rec.Lines {
		pref := prefs[l.PositionID]
		lines[i] = receiptLineDTO{
			PositionID:   l.PositionID,
			PositionName: pref.Name,
			LotNumber:    pref.LotNumber,
			BrandName:    pref.BrandName,
			Quantity:     l.Quantity,
		}
	}
	return receiptResponse{
		ID:            rec.ID,
		Date:          rec.Date,
		Note:          rec.Note,
		SupplierID:    rec.SupplierID,
		SupplierName:  supplierNames[rec.SupplierID],
		Counterparty:  rec.Counterparty,
		InvoiceAmount: rec.InvoiceAmount,
		Lines:         lines,
		CreatedBy:     rec.CreatedBy,
		CreatedAt:     rec.CreatedAt,
	}
}

type receiptLineRequest struct {
	PositionID string `json:"position_id"`
	Quantity   int    `json:"quantity"`
}

type createReceiptRequest struct {
	Date          time.Time            `json:"date"`
	Note          string               `json:"note"`
	SupplierID    string               `json:"supplier_id"`
	InvoiceAmount int64                `json:"invoice_amount"`
	Lines         []receiptLineRequest `json:"lines"`
}

func (h *ReceiptHandler) List(w http.ResponseWriter, r *http.Request) {
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

	receipts, total, err := h.receipts.List(r.Context(), domain.Filter{
		PositionID: r.URL.Query().Get("position_id"),
		SupplierID: r.URL.Query().Get("supplier_id"),
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
	prefs, err := h.receipts.PositionRefs(r.Context(), receipts)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	supplierNames, err := h.receipts.SupplierNames(r.Context(), receipts)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]receiptResponse, len(receipts))
	for i := range receipts {
		items[i] = toReceiptResponse(&receipts[i], prefs, supplierNames)
	}
	web.List(w, items, total, p)
}

func (h *ReceiptHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReceiptRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	lines := make([]receiptsvc.LineInput, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = receiptsvc.LineInput{PositionID: l.PositionID, Quantity: l.Quantity}
	}

	rec, err := h.receipts.Create(r.Context(), receiptsvc.CreateInput{
		Date:          req.Date,
		Note:          req.Note,
		SupplierID:    req.SupplierID,
		InvoiceAmount: req.InvoiceAmount,
		Lines:         lines,
		CreatedBy:     actorID(actor),
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	h.respondReceipt(w, r, http.StatusCreated, rec)
}

func (h *ReceiptHandler) Get(w http.ResponseWriter, r *http.Request) {
	rec, err := h.receipts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	h.respondReceipt(w, r, http.StatusOK, rec)
}

// respondReceipt enriches and writes a single receipt.
func (h *ReceiptHandler) respondReceipt(w http.ResponseWriter, r *http.Request, status int, rec *domain.Receipt) {
	prefs, err := h.receipts.PositionRefs(r.Context(), []domain.Receipt{*rec})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	supplierNames, err := h.receipts.SupplierNames(r.Context(), []domain.Receipt{*rec})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, status, toReceiptResponse(rec, prefs, supplierNames))
}
