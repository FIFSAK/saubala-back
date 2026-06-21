package shipment

import (
	"fmt"
	"time"
)

type Shipment struct {
	ID              string
	ReferenceNumber string
	Origin          string
	Destination     string
	Status          Status
	DriverName      string
	UnitNumber      string
	ShipmentAmount  float64
	DriverRevenue   float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Event struct {
	ID         string
	ShipmentID string
	Status     Status
	Comment    string
	CreatedAt  time.Time
}

func NewShipment(referenceNumber, origin, destination, driverName, unitNumber string, amount, revenue float64) (Shipment, error) {
	if referenceNumber == "" {
		return Shipment{}, fmt.Errorf("reference number is required")
	}
	if origin == "" {
		return Shipment{}, fmt.Errorf("origin is required")
	}
	if destination == "" {
		return Shipment{}, fmt.Errorf("destination is required")
	}

	now := time.Now()
	return Shipment{
		ReferenceNumber: referenceNumber,
		Origin:          origin,
		Destination:     destination,
		Status:          StatusPending,
		DriverName:      driverName,
		UnitNumber:      unitNumber,
		ShipmentAmount:  amount,
		DriverRevenue:   revenue,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}
