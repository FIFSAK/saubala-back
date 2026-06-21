CREATE TABLE IF NOT EXISTS shipments (
    id              TEXT PRIMARY KEY,
    reference_number TEXT NOT NULL,
    origin          TEXT NOT NULL,
    destination     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    driver_name     TEXT NOT NULL DEFAULT '',
    unit_number     TEXT NOT NULL DEFAULT '',
    shipment_amount REAL NOT NULL DEFAULT 0,
    driver_revenue  REAL NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS shipment_events (
    id          TEXT PRIMARY KEY,
    shipment_id TEXT NOT NULL REFERENCES shipments(id),
    status      TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_shipment_events_shipment_id ON shipment_events(shipment_id);
