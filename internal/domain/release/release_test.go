package release

import (
	"strings"
	"testing"
	"time"
)

func validLines() []Line {
	return []Line{{ContractLineID: "cl-1", PositionID: "pos-1", Quantity: 2}}
}

func TestNewContractRelease(t *testing.T) {
	r, err := New(NewInput{
		ContractID:       " c-1 ",
		Date:             time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Note:             " note ",
		DocumentNumber:   " 42 ",
		RecipientName:    "ГКП «Центр»",
		RecipientAddress: "г. Астана",
		OrganizationID:   "org-1",
		CreatedBy:        "u-1",
		Lines:            validLines(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ContractID != "c-1" || r.DocumentNumber != "42" || r.Note != "note" {
		t.Fatalf("fields not trimmed: %+v", r)
	}
}

func TestNewFreeRelease(t *testing.T) {
	r, err := New(NewInput{
		Date:             time.Now(),
		DocumentNumber:   "7",
		RecipientName:    "Получатель",
		RecipientAddress: "Адрес",
		Lines:            []Line{{PositionID: "pos-1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ContractID != "" {
		t.Fatalf("expected empty contract id, got %q", r.ContractID)
	}
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name string
		in   NewInput
		want string
	}{
		{
			name: "no date",
			in:   NewInput{ContractID: "c-1", Lines: validLines()},
			want: "дата",
		},
		{
			name: "no lines",
			in:   NewInput{ContractID: "c-1", Date: time.Now()},
			want: "хотя бы одна строка",
		},
		{
			name: "contract release without contract line",
			in: NewInput{ContractID: "c-1", Date: time.Now(),
				Lines: []Line{{PositionID: "pos-1", Quantity: 1}}},
			want: "строка договора обязательна",
		},
		{
			name: "free release with contract line",
			in: NewInput{Date: time.Now(),
				Lines: []Line{{ContractLineID: "cl-1", PositionID: "pos-1", Quantity: 1}}},
			want: "без договора",
		},
		{
			name: "zero quantity",
			in: NewInput{ContractID: "c-1", Date: time.Now(),
				Lines: []Line{{ContractLineID: "cl-1", PositionID: "pos-1", Quantity: 0}}},
			want: "количество",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}
