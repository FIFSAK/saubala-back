package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FIFSAK/saubala-back/internal/domain/shipment"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type ShipmentRepository struct {
	db *sql.DB
}

func NewShipmentRepository(db *sql.DB) *ShipmentRepository {
	return &ShipmentRepository{db: db}
}

func (r *ShipmentRepository) Create(ctx context.Context, s shipment.Shipment) (string, error) {
	id := uuid.New().String()

	query := `INSERT INTO shipments (id, reference_number, origin, destination, status, driver_name, unit_number, shipment_amount, driver_revenue, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		id,
		s.ReferenceNumber,
		s.Origin,
		s.Destination,
		string(s.Status),
		s.DriverName,
		s.UnitNumber,
		s.ShipmentAmount,
		s.DriverRevenue,
		s.CreatedAt.Format(time.RFC3339Nano),
		s.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("insert shipment: %w", err)
	}

	return id, nil
}

func (r *ShipmentRepository) GetByID(ctx context.Context, id string) (shipment.Shipment, error) {
	query := `SELECT id, reference_number, origin, destination, status, driver_name, unit_number, shipment_amount, driver_revenue, created_at, updated_at
		FROM shipments WHERE id = ?`

	var s shipment.Shipment
	var status string
	var createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID,
		&s.ReferenceNumber,
		&s.Origin,
		&s.Destination,
		&status,
		&s.DriverName,
		&s.UnitNumber,
		&s.ShipmentAmount,
		&s.DriverRevenue,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return shipment.Shipment{}, store.ErrorNotFound
		}
		return shipment.Shipment{}, fmt.Errorf("get shipment: %w", err)
	}

	s.Status = shipment.Status(status)
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return s, nil
}

func (r *ShipmentRepository) List(ctx context.Context) ([]shipment.Shipment, error) {
	query := `SELECT id, reference_number, origin, destination, status, driver_name, unit_number, shipment_amount, driver_revenue, created_at, updated_at
		FROM shipments ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list shipments: %w", err)
	}
	defer rows.Close()

	var shipments []shipment.Shipment
	for rows.Next() {
		var s shipment.Shipment
		var status string
		var createdAt, updatedAt string
		if err := rows.Scan(
			&s.ID,
			&s.ReferenceNumber,
			&s.Origin,
			&s.Destination,
			&status,
			&s.DriverName,
			&s.UnitNumber,
			&s.ShipmentAmount,
			&s.DriverRevenue,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan shipment: %w", err)
		}
		s.Status = shipment.Status(status)
		s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		shipments = append(shipments, s)
	}

	return shipments, nil
}

func (r *ShipmentRepository) UpdateStatus(ctx context.Context, id string, status shipment.Status) error {
	query := `UPDATE shipments SET status = ?, updated_at = ? WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, string(status), time.Now().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return store.ErrorNotFound
	}

	return nil
}

func (r *ShipmentRepository) AddEvent(ctx context.Context, event shipment.Event) (string, error) {
	id := uuid.New().String()

	query := `INSERT INTO shipment_events (id, shipment_id, status, comment, created_at)
		VALUES (?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		id,
		event.ShipmentID,
		string(event.Status),
		event.Comment,
		event.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("insert event: %w", err)
	}

	return id, nil
}

func (r *ShipmentRepository) ListEvents(ctx context.Context, shipmentID string) ([]shipment.Event, error) {
	query := `SELECT id, shipment_id, status, comment, created_at
		FROM shipment_events WHERE shipment_id = ? ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []shipment.Event
	for rows.Next() {
		var e shipment.Event
		var status string
		var createdAt string
		if err := rows.Scan(&e.ID, &e.ShipmentID, &status, &e.Comment, &createdAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Status = shipment.Status(status)
		e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		events = append(events, e)
	}

	return events, nil
}
