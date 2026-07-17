package position

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	expiry := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)

	p, err := New("Milk", "brand-1", "sup-1", "Milk (contract)", "LOT-1", expiry, 50000, 10, 250)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Error("expected generated id")
	}
	if p.Quantity != 10 {
		t.Errorf("quantity = %d, want 10", p.Quantity)
	}
	if p.SupplierID != "sup-1" {
		t.Errorf("supplier = %q, want sup-1", p.SupplierID)
	}
	if !p.ExpiryDate.Equal(expiry) {
		t.Errorf("expiry not preserved: %v", p.ExpiryDate)
	}

	bad := []struct {
		name string
		fn   func() (*Position, error)
	}{
		{"empty name", func() (*Position, error) { return New("", "b", "", "", "L", expiry, 0, 0, 0) }},
		{"empty brand", func() (*Position, error) { return New("n", "", "", "", "L", expiry, 0, 0, 0) }},
		{"zero expiry", func() (*Position, error) { return New("n", "b", "", "", "L", time.Time{}, 0, 0, 0) }},
		{"empty lot", func() (*Position, error) { return New("n", "b", "", "", "", expiry, 0, 0, 0) }},
		{"negative price", func() (*Position, error) { return New("n", "b", "", "", "L", expiry, -1, 0, 0) }},
		{"negative mass", func() (*Position, error) { return New("n", "b", "", "", "L", expiry, 0, 0, -1) }},
		{"negative qty", func() (*Position, error) { return New("n", "b", "", "", "L", expiry, 0, -1, 0) }},
	}
	for _, tc := range bad {
		if _, err := tc.fn(); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}
