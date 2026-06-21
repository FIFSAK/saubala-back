package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/shipment"
	shipmentSvc "github.com/FIFSAK/saubala-back/internal/service/shipment"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type ShipmentHandler struct {
	service shipmentSvc.Service
}

func NewShipmentHandler(service shipmentSvc.Service) *ShipmentHandler {
	return &ShipmentHandler{service: service}
}

// Register wires the shipment routes onto the given router.
func (h *ShipmentHandler) Register(r chi.Router) {
	r.Post("/shipments", h.CreateShipment)
	r.Get("/shipments", h.ListShipments)
	r.Get("/shipments/{id}", h.GetShipment)
	r.Post("/shipments/{id}/events", h.AddShipmentEvent)
	r.Get("/shipments/{id}/events", h.GetShipmentEvents)
}

type createShipmentRequest struct {
	ReferenceNumber string  `json:"reference_number"`
	Origin          string  `json:"origin"`
	Destination     string  `json:"destination"`
	DriverName      string  `json:"driver_name"`
	UnitNumber      string  `json:"unit_number"`
	ShipmentAmount  float64 `json:"shipment_amount"`
	DriverRevenue   float64 `json:"driver_revenue"`
}

type addEventRequest struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
}

type shipmentResponse struct {
	ID              string    `json:"id"`
	ReferenceNumber string    `json:"reference_number"`
	Origin          string    `json:"origin"`
	Destination     string    `json:"destination"`
	Status          string    `json:"status"`
	DriverName      string    `json:"driver_name"`
	UnitNumber      string    `json:"unit_number"`
	ShipmentAmount  float64   `json:"shipment_amount"`
	DriverRevenue   float64   `json:"driver_revenue"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type eventResponse struct {
	ID         string    `json:"id"`
	ShipmentID string    `json:"shipment_id"`
	Status     string    `json:"status"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *ShipmentHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	var req createShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s, err := h.service.CreateShipment(r.Context(), shipmentSvc.CreateRequest{
		ReferenceNumber: req.ReferenceNumber,
		Origin:          req.Origin,
		Destination:     req.Destination,
		DriverName:      req.DriverName,
		UnitNumber:      req.UnitNumber,
		ShipmentAmount:  req.ShipmentAmount,
		DriverRevenue:   req.DriverRevenue,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toShipmentResponse(s))
}

func (h *ShipmentHandler) GetShipment(w http.ResponseWriter, r *http.Request) {
	s, err := h.service.GetShipment(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			writeError(w, http.StatusNotFound, "shipment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toShipmentResponse(s))
}

func (h *ShipmentHandler) ListShipments(w http.ResponseWriter, r *http.Request) {
	shipments, err := h.service.ListShipments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]shipmentResponse, len(shipments))
	for i, s := range shipments {
		resp[i] = toShipmentResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *ShipmentHandler) AddShipmentEvent(w http.ResponseWriter, r *http.Request) {
	var req addEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status := domain.Status(req.Status)
	if !status.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	event, err := h.service.AddEvent(r.Context(), chi.URLParam(r, "id"), status, req.Comment)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			writeError(w, http.StatusNotFound, "shipment not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toEventResponse(event))
}

func (h *ShipmentHandler) GetShipmentEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.GetEvents(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			writeError(w, http.StatusNotFound, "shipment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = toEventResponse(e)
	}

	writeJSON(w, http.StatusOK, resp)
}

func toShipmentResponse(s domain.Shipment) shipmentResponse {
	return shipmentResponse{
		ID:              s.ID,
		ReferenceNumber: s.ReferenceNumber,
		Origin:          s.Origin,
		Destination:     s.Destination,
		Status:          string(s.Status),
		DriverName:      s.DriverName,
		UnitNumber:      s.UnitNumber,
		ShipmentAmount:  s.ShipmentAmount,
		DriverRevenue:   s.DriverRevenue,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func toEventResponse(e domain.Event) eventResponse {
	return eventResponse{
		ID:         e.ID,
		ShipmentID: e.ShipmentID,
		Status:     string(e.Status),
		Comment:    e.Comment,
		CreatedAt:  e.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
