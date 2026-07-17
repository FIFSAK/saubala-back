package invoice

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func sampleInvoice(n int) *Invoice {
	lines := make([]LineInput, n)
	for i := range lines {
		lines[i] = LineInput{
			Name:      fmt.Sprintf("Продукт специализированный, для энтерального питания(Позиция %d, Срок годности: 27.03.2027)", i+1),
			Unit:      "штука",
			Quantity:  i + 1,
			UnitPrice: 928000,
		}
	}
	return Build(BuildInput{
		DocumentNumber:       "404",
		DocumentDate:         time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
		SellerName:           "ТОО «Смак-МН»",
		SellerBIN:            "201140002658",
		ResponsibleForSupply: "Руководитель Турсынбекова Айгерим Аманжоловна",
		Director:             "Руководитель Турсынбекова Айгерим",
		Accountant:           "Не предусмотрен",
		RecipientName:        "ГКП Городская поликлиника № 2",
		RecipientAddress:     "г.Шымкент, пр. Абая, 35",
		VATRatePercent:       16,
		Lines:                lines,
	})
}

func TestRenderXLSXRowResize(t *testing.T) {
	for _, n := range []int{1, 3, 5} {
		inv := sampleInvoice(n)
		data, err := RenderXLSX(inv)
		if err != nil {
			t.Fatalf("n=%d RenderXLSX: %v", n, err)
		}
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("n=%d reopen: %v", n, err)
		}

		// Header cells never shift.
		if got, _ := f.GetCellValue(xlsxSheet, "AP13"); got != "404" {
			t.Errorf("n=%d document number = %q, want 404", n, got)
		}
		if got, _ := f.GetCellValue(xlsxSheet, "L19"); got != inv.RecipientName {
			t.Errorf("n=%d recipient = %q", n, got)
		}

		// Each item row is present with the right quantity.
		for i := range n {
			row := xlsxFirstDataRow + i
			got, _ := f.GetCellValue(xlsxSheet, cell("W", row))
			if got != fmt.Sprintf("%d", i+1) {
				t.Errorf("n=%d row %d qty = %q, want %d", n, row, got, i+1)
			}
		}

		// Totals land on the shifted row and carry the grand totals.
		totalsRow := xlsxFirstDataRow + n
		gotSum, _ := f.GetCellValue(xlsxSheet, cell("AL", totalsRow))
		if gotSum != formatTenge(inv.TotalSum) {
			t.Errorf("n=%d totals sum = %q, want %q", n, gotSum, formatTenge(inv.TotalSum))
		}
		// Amount in words on the following row.
		gotWords, _ := f.GetCellValue(xlsxSheet, cell("AE", totalsRow+1))
		if gotWords != inv.AmountInWords {
			t.Errorf("n=%d words = %q, want %q", n, gotWords, inv.AmountInWords)
		}
		_ = f.Close()
	}
}

func TestRenderPDFValid(t *testing.T) {
	data, err := RenderPDF(sampleInvoice(3))
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("output is not a PDF (prefix %q)", data[:min(8, len(data))])
	}
	if len(data) < 1000 {
		t.Fatalf("PDF suspiciously small: %d bytes", len(data))
	}
}
