// Package invoice builds and renders the release waybill «Накладная на отпуск
// запасов на сторону» (Форма З-2) in XLSX and PDF.
//
// Money is int64 tiyn (1 ₸ = 100 tiyn) throughout; the renderers convert to
// tenge only for display. Prices are treated as VAT-inclusive: the VAT column is
// the tax embedded in the gross sum, vat = round(sum * rate / (100 + rate)).
package invoice

import (
	"fmt"
	"strings"
	"time"
)

// Line is a computed waybill row.
type Line struct {
	Index     int    // 1-based «номер по порядку»
	Name      string // «наименование, характеристика»
	Unit      string // «единица измерения»
	Quantity  int    // «количество»
	UnitPrice int64  // «цена за единицу», tiyn, VAT-inclusive
	Sum       int64  // «сумма с НДС», tiyn
	VAT       int64  // «в том числе сумма НДС», tiyn
}

// Invoice is the fully-computed waybill, ready to render.
type Invoice struct {
	DocumentNumber string
	DocumentDate   time.Time

	SellerName           string
	SellerBIN            string
	ResponsibleForSupply string
	Director             string
	Accountant           string

	RecipientName    string
	RecipientAddress string

	VATRatePercent int
	Lines          []Line

	TotalQuantity int
	TotalSum      int64
	TotalVAT      int64
	AmountInWords string
}

// LineInput is one resolved row handed to Build (prices already chosen by the
// service from the contract line / position).
type LineInput struct {
	Name      string
	Unit      string
	Quantity  int
	UnitPrice int64 // tiyn, VAT-inclusive
}

// BuildInput is the fully-resolved data used to compute an Invoice.
type BuildInput struct {
	DocumentNumber string
	DocumentDate   time.Time

	SellerName           string
	SellerBIN            string
	ResponsibleForSupply string
	Director             string
	Accountant           string

	RecipientName    string
	RecipientAddress string

	VATRatePercent int
	Lines          []LineInput
}

// Build computes line sums, embedded VAT, totals and the amount-in-words.
func Build(in BuildInput) *Invoice {
	inv := &Invoice{
		DocumentNumber:       in.DocumentNumber,
		DocumentDate:         in.DocumentDate,
		SellerName:           in.SellerName,
		SellerBIN:            in.SellerBIN,
		ResponsibleForSupply: in.ResponsibleForSupply,
		Director:             in.Director,
		Accountant:           in.Accountant,
		RecipientName:        in.RecipientName,
		RecipientAddress:     in.RecipientAddress,
		VATRatePercent:       in.VATRatePercent,
	}
	for i, li := range in.Lines {
		sum := int64(li.Quantity) * li.UnitPrice
		vat := vatInclusive(sum, in.VATRatePercent)
		inv.Lines = append(inv.Lines, Line{
			Index:     i + 1,
			Name:      li.Name,
			Unit:      li.Unit,
			Quantity:  li.Quantity,
			UnitPrice: li.UnitPrice,
			Sum:       sum,
			VAT:       vat,
		})
		inv.TotalQuantity += li.Quantity
		inv.TotalSum += sum
		inv.TotalVAT += vat
	}
	inv.AmountInWords = TengeInWords(inv.TotalSum)
	return inv
}

// vatInclusive returns the VAT embedded in a VAT-inclusive gross sum at the given
// rate, rounded to the nearest tiyn: vat = round(sum * rate / (100 + rate)).
func vatInclusive(sum int64, ratePercent int) int64 {
	if ratePercent <= 0 || sum <= 0 {
		return 0
	}
	denom := int64(100 + ratePercent)
	return (sum*int64(ratePercent) + denom/2) / denom
}

// tengeFloat converts tiyn to a tenge float for numeric spreadsheet cells.
func tengeFloat(tiyn int64) float64 { return float64(tiyn) / 100 }

// formatTenge renders tiyn as a tenge string with a space thousands separator and
// two decimals, e.g. 33872000 -> "338 720.00". Used by the PDF renderer.
func formatTenge(tiyn int64) string {
	neg := tiyn < 0
	if neg {
		tiyn = -tiyn
	}
	whole := tiyn / 100
	frac := tiyn % 100
	s := groupThousands(whole)
	out := fmt.Sprintf("%s.%02d", s, frac)
	if neg {
		out = "-" + out
	}
	return out
}

// groupThousands inserts a space every three digits from the right.
func groupThousands(n int64) string {
	digits := fmt.Sprintf("%d", n)
	var b strings.Builder
	rem := len(digits) % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(digits[:rem])
	for i := rem; i < len(digits); i += 3 {
		b.WriteByte(' ')
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
