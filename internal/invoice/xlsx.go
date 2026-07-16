package invoice

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/xuri/excelize/v2"
)

//go:embed assets/template.xlsx
var xlsxTemplate []byte

const xlsxSheet = "TDSheet"

// Template layout (see the customer's Форма З-2 workbook). The header block sits
// above the item table and never shifts; the item table has three data rows
// (24..26), a totals row (27), a quantity/amount-in-words row (28) and a
// signature block below. When the line count differs from three we insert or
// remove rows in the item table and everything from the totals row down shifts.
const (
	xlsxFirstDataRow = 24
	xlsxTemplateRows = 3 // data rows present in the template
	xlsxTotalsRow    = 27
	xlsxWordsRow     = 28
	xlsxDirectorRow  = 30
	xlsxAccountRow   = 34
)

// dataRowMerges are the nine merged ranges of a single item row (start..end
// columns), recreated on every inserted row.
var dataRowMerges = [9][2]string{
	{"A", "B"}, {"C", "N"}, {"O", "S"}, {"T", "V"}, {"W", "AA"},
	{"AB", "AE"}, {"AF", "AK"}, {"AL", "AQ"}, {"AR", "AW"},
}

// RenderXLSX fills the embedded Форма З-2 template with the invoice data and
// returns the resulting workbook bytes. Styles, borders and merges come from the
// template, so the output is visually identical to the customer's file.
func RenderXLSX(inv *Invoice) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(xlsxTemplate))
	if err != nil {
		return nil, fmt.Errorf("open invoice template: %w", err)
	}
	defer f.Close()

	n := len(inv.Lines)
	shift, err := resizeItemRows(f, n)
	if err != nil {
		return nil, err
	}

	// Header block (fixed rows, never shifts).
	set := func(cell, val string) { _ = f.SetCellStr(xlsxSheet, cell, val) }
	set("N9", inv.SellerName)
	set("AQ9", inv.SellerBIN)
	set("AP13", inv.DocumentNumber)
	set("AT13", inv.DocumentDate.Format("02.01.2006"))
	set("A19", inv.SellerName)
	set("L19", inv.RecipientName)
	set("W19", inv.ResponsibleForSupply)
	set("AO19", inv.RecipientAddress)

	// Item rows.
	for i, line := range inv.Lines {
		row := xlsxFirstDataRow + i
		_ = f.SetCellInt(xlsxSheet, cell("A", row), int64(line.Index))
		_ = f.SetCellStr(xlsxSheet, cell("C", row), line.Name)
		_ = f.SetCellInt(xlsxSheet, cell("O", row), int64(line.Index))
		_ = f.SetCellStr(xlsxSheet, cell("T", row), line.Unit)
		_ = f.SetCellInt(xlsxSheet, cell("W", row), int64(line.Quantity))
		_ = f.SetCellInt(xlsxSheet, cell("AB", row), int64(line.Quantity))
		_ = f.SetCellStr(xlsxSheet, cell("AF", row), formatTenge(line.UnitPrice))
		_ = f.SetCellStr(xlsxSheet, cell("AL", row), formatTenge(line.Sum))
		_ = f.SetCellStr(xlsxSheet, cell("AR", row), formatTenge(line.VAT))
	}

	// Totals row.
	totals := xlsxTotalsRow + shift
	_ = f.SetCellInt(xlsxSheet, cell("W", totals), int64(inv.TotalQuantity))
	_ = f.SetCellInt(xlsxSheet, cell("AB", totals), int64(inv.TotalQuantity))
	_ = f.SetCellStr(xlsxSheet, cell("AL", totals), formatTenge(inv.TotalSum))
	_ = f.SetCellStr(xlsxSheet, cell("AR", totals), formatTenge(inv.TotalVAT))

	// Quantity + amount-in-words row.
	words := xlsxWordsRow + shift
	_ = f.SetCellInt(xlsxSheet, cell("N", words), int64(inv.TotalQuantity))
	_ = f.SetCellStr(xlsxSheet, cell("AE", words), inv.AmountInWords)

	// Signature block.
	_ = f.SetCellStr(xlsxSheet, cell("R", xlsxDirectorRow+shift), inv.Director)
	_ = f.SetCellStr(xlsxSheet, cell("L", xlsxAccountRow+shift), inv.Accountant)

	var out bytes.Buffer
	if err := f.Write(&out); err != nil {
		return nil, fmt.Errorf("write invoice workbook: %w", err)
	}
	return out.Bytes(), nil
}

// resizeItemRows grows or shrinks the item table to hold n rows and returns the
// row offset (n-3) applied to everything below the table.
func resizeItemRows(f *excelize.File, n int) (int, error) {
	switch {
	case n > xlsxTemplateRows:
		extra := n - xlsxTemplateRows
		// Insert blank rows just before the totals row, then clone the first data
		// row's height, per-cell styles and merges onto each new row.
		if err := f.InsertRows(xlsxSheet, xlsxTotalsRow, extra); err != nil {
			return 0, fmt.Errorf("insert item rows: %w", err)
		}
		h, err := f.GetRowHeight(xlsxSheet, xlsxFirstDataRow)
		if err != nil {
			return 0, err
		}
		for r := xlsxFirstDataRow + xlsxTemplateRows; r < xlsxFirstDataRow+n; r++ {
			if err := cloneDataRow(f, xlsxFirstDataRow, r, h); err != nil {
				return 0, err
			}
		}
	case n < xlsxTemplateRows:
		// Remove surplus template data rows from the bottom of the block upward, so
		// the remaining row indices stay valid as rows below shift up.
		for r := xlsxFirstDataRow + xlsxTemplateRows - 1; r >= xlsxFirstDataRow+n; r-- {
			if err := f.RemoveRow(xlsxSheet, r); err != nil {
				return 0, fmt.Errorf("remove item row: %w", err)
			}
		}
	}
	return n - xlsxTemplateRows, nil
}

// cloneDataRow copies the height, per-column cell styles and merged ranges of the
// template data row src onto row dst.
func cloneDataRow(f *excelize.File, src, dst int, height float64) error {
	if err := f.SetRowHeight(xlsxSheet, dst, height); err != nil {
		return err
	}
	for c := 1; c <= 49; c++ { // columns A..AW
		from, _ := excelize.CoordinatesToCellName(c, src)
		to, _ := excelize.CoordinatesToCellName(c, dst)
		style, err := f.GetCellStyle(xlsxSheet, from)
		if err != nil {
			return err
		}
		if err := f.SetCellStyle(xlsxSheet, to, to, style); err != nil {
			return err
		}
	}
	for _, m := range dataRowMerges {
		if err := f.MergeCell(xlsxSheet, cell(m[0], dst), cell(m[1], dst)); err != nil {
			return err
		}
	}
	return nil
}

func cell(col string, row int) string { return fmt.Sprintf("%s%d", col, row) }
