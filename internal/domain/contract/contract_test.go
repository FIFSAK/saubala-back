package contract

import (
	"testing"
	"time"
)

func TestValidateBIN(t *testing.T) {
	valid := []string{"123456789012", "000000000000"}
	invalid := []string{"", "12345", "1234567890123", "12345678901a", "abcdefghijkl"}
	for _, b := range valid {
		if err := ValidateBIN(b); err != nil {
			t.Errorf("ValidateBIN(%q) unexpected error: %v", b, err)
		}
	}
	for _, b := range invalid {
		if err := ValidateBIN(b); err == nil {
			t.Errorf("ValidateBIN(%q) expected error, got nil", b)
		}
	}
}

func validLines() []Line {
	return []Line{{PositionID: "pos-1", PlannedQuantity: 10}}
}

func TestNew(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	c, err := New("Plan", "ГКП «План»", "Almaty", "C-1", date, "123456789012", "user-1", validLines())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID == "" {
		t.Error("expected generated contract id")
	}
	if len(c.Lines) != 1 || c.Lines[0].ID == "" {
		t.Error("expected line id to be generated")
	}

	bad := []struct {
		name string
		fn   func() (*Contract, error)
	}{
		{"empty name", func() (*Contract, error) { return New("", "", "a", "C-1", date, "123456789012", "u", validLines()) }},
		{"empty address", func() (*Contract, error) { return New("n", "", "", "C-1", date, "123456789012", "u", validLines()) }},
		{"empty number", func() (*Contract, error) { return New("n", "", "a", "", date, "123456789012", "u", validLines()) }},
		{"zero date", func() (*Contract, error) {
			return New("n", "", "a", "C-1", time.Time{}, "123456789012", "u", validLines())
		}},
		{"bad bin", func() (*Contract, error) { return New("n", "", "a", "C-1", date, "123", "u", validLines()) }},
		{"no lines", func() (*Contract, error) { return New("n", "", "a", "C-1", date, "123456789012", "u", nil) }},
		{"bad qty", func() (*Contract, error) {
			return New("n", "", "a", "C-1", date, "123456789012", "u", []Line{{PositionID: "p", PlannedQuantity: 0}})
		}},
		{"empty position", func() (*Contract, error) {
			return New("n", "", "a", "C-1", date, "123456789012", "u", []Line{{PositionID: "", PlannedQuantity: 1}})
		}},
	}
	for _, tc := range bad {
		if _, err := tc.fn(); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestNormalizeLinesPreservesID(t *testing.T) {
	lines := []Line{{ID: "fixed", PositionID: "p", PlannedQuantity: 2}}
	out, err := NormalizeLines(lines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[0].ID != "fixed" {
		t.Errorf("expected preserved id 'fixed', got %q", out[0].ID)
	}

	gen, err := NormalizeLines([]Line{{PositionID: "p", PlannedQuantity: 2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gen[0].ID == "" {
		t.Error("expected generated id for line without one")
	}
}
