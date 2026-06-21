package shipment

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	domain "github.com/FIFSAK/saubala-back/internal/domain/shipment"
	"github.com/FIFSAK/saubala-back/pkg/log"
)

type Service interface {
	CreateShipment(ctx context.Context, req CreateRequest) (domain.Shipment, error)
	GetShipment(ctx context.Context, id string) (domain.Shipment, error)
	ListShipments(ctx context.Context) ([]domain.Shipment, error)
	AddEvent(ctx context.Context, shipmentID string, status domain.Status, comment string) (domain.Event, error)
	GetEvents(ctx context.Context, shipmentID string) ([]domain.Event, error)
}

type CreateRequest struct {
	ReferenceNumber string
	Origin          string
	Destination     string
	DriverName      string
	UnitNumber      string
	ShipmentAmount  float64
	DriverRevenue   float64
}

type ShipmentService struct {
	repo domain.Repository
}

func NewShipmentService(repo domain.Repository) *ShipmentService {
	return &ShipmentService{repo: repo}
}

func (s *ShipmentService) CreateShipment(ctx context.Context, req CreateRequest) (domain.Shipment, error) {
	logger := log.FromContext(ctx).Named("create_shipment")

	shipment, err := domain.NewShipment(
		req.ReferenceNumber,
		req.Origin,
		req.Destination,
		req.DriverName,
		req.UnitNumber,
		req.ShipmentAmount,
		req.DriverRevenue,
	)
	if err != nil {
		logger.Warn("validation failed", zap.Error(err))
		return domain.Shipment{}, fmt.Errorf("validation: %w", err)
	}

	id, err := s.repo.Create(ctx, shipment)
	if err != nil {
		logger.Error("failed to create shipment", zap.Error(err))
		return domain.Shipment{}, fmt.Errorf("create shipment: %w", err)
	}
	shipment.ID = id

	initialEvent := domain.Event{
		ShipmentID: id,
		Status:     domain.StatusPending,
		Comment:    "shipment created",
		CreatedAt:  time.Now(),
	}
	if _, err := s.repo.AddEvent(ctx, initialEvent); err != nil {
		logger.Error("failed to add initial event", zap.Error(err))
		return domain.Shipment{}, fmt.Errorf("add initial event: %w", err)
	}

	logger.Info("shipment created", zap.String("id", id))
	return shipment, nil
}

func (s *ShipmentService) GetShipment(ctx context.Context, id string) (domain.Shipment, error) {
	logger := log.FromContext(ctx).Named("get_shipment")

	shipment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.Error("failed to get shipment", zap.String("id", id), zap.Error(err))
		return domain.Shipment{}, err
	}

	return shipment, nil
}

func (s *ShipmentService) ListShipments(ctx context.Context) ([]domain.Shipment, error) {
	logger := log.FromContext(ctx).Named("list_shipments")

	shipments, err := s.repo.List(ctx)
	if err != nil {
		logger.Error("failed to list shipments", zap.Error(err))
		return nil, err
	}

	return shipments, nil
}

func (s *ShipmentService) AddEvent(ctx context.Context, shipmentID string, status domain.Status, comment string) (domain.Event, error) {
	logger := log.FromContext(ctx).Named("add_event")

	current, err := s.repo.GetByID(ctx, shipmentID)
	if err != nil {
		logger.Error("shipment not found", zap.String("id", shipmentID), zap.Error(err))
		return domain.Event{}, fmt.Errorf("get shipment: %w", err)
	}

	if err := current.Status.ValidateTransition(status); err != nil {
		logger.Warn("invalid status transition",
			zap.String("from", string(current.Status)),
			zap.String("to", string(status)),
			zap.Error(err),
		)
		return domain.Event{}, err
	}

	event := domain.Event{
		ShipmentID: shipmentID,
		Status:     status,
		Comment:    comment,
		CreatedAt:  time.Now(),
	}

	eventID, err := s.repo.AddEvent(ctx, event)
	if err != nil {
		logger.Error("failed to add event", zap.Error(err))
		return domain.Event{}, fmt.Errorf("add event: %w", err)
	}
	event.ID = eventID

	if err := s.repo.UpdateStatus(ctx, shipmentID, status); err != nil {
		logger.Error("failed to update shipment status", zap.Error(err))
		return domain.Event{}, fmt.Errorf("update status: %w", err)
	}

	logger.Info("event added",
		zap.String("shipment_id", shipmentID),
		zap.String("status", string(status)),
	)
	return event, nil
}

func (s *ShipmentService) GetEvents(ctx context.Context, shipmentID string) ([]domain.Event, error) {
	logger := log.FromContext(ctx).Named("get_events")

	if _, err := s.repo.GetByID(ctx, shipmentID); err != nil {
		logger.Error("shipment not found", zap.String("id", shipmentID), zap.Error(err))
		return nil, fmt.Errorf("get shipment: %w", err)
	}

	events, err := s.repo.ListEvents(ctx, shipmentID)
	if err != nil {
		logger.Error("failed to list events", zap.Error(err))
		return nil, err
	}

	return events, nil
}
