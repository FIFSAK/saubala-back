package shipment

import "context"

type Repository interface {
	Create(ctx context.Context, shipment Shipment) (string, error)
	GetByID(ctx context.Context, id string) (Shipment, error)
	List(ctx context.Context) ([]Shipment, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	AddEvent(ctx context.Context, event Event) (string, error)
	ListEvents(ctx context.Context, shipmentID string) ([]Event, error)
}
