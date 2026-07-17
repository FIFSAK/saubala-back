package invoice

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/go-pdf/fpdf"
)

//go:embed assets/DejaVuSansCondensed.ttf
var pdfFontRegular []byte

//go:embed assets/DejaVuSansCondensed-Bold.ttf
var pdfFontBold []byte

const (
	pdfFont         = "DejaVu"
	pdfLineH        = 4.0 // mm per text line inside a cell
	pdfLeft         = 10.0
	pdfRight        = 10.0
	pdfTop          = 10.0
	pdfBottomMargin = 15.0 // A4 portrait bottom margin (mm)
	pdfTableWrap    = 1    // index of the wrapping column («Наименование»)
)

// tableCols defines the nine item-table columns: header text, width (mm) and
// horizontal alignment.
type tableCol struct {
	head  string
	width float64
	align string
}

var tableCols = []tableCol{
	{"№", 8, "C"},
	{"Наименование, характеристика", 62, "L"},
	{"Номенкл. №", 13, "C"},
	{"Ед. изм.", 13, "C"},
	{"Подлежит отпуску", 14, "C"},
	{"Отпущено", 14, "C"},
	{"Цена за ед., ₸", 22, "R"},
	{"Сумма с НДС, ₸", 24, "R"},
	{"в т.ч. НДС, ₸", 20, "R"},
}

// RenderPDF renders the waybill as a PDF laid out to resemble the official
// Форма З-2 (appendix reference, form code, document-number box, bordered item
// table, totals, amount-in-words and signature block).
func RenderPDF(inv *Invoice) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pdfLeft, pdfTop, pdfRight)
	pdf.SetAutoPageBreak(true, pdfBottomMargin)
	pdf.AddUTF8FontFromBytes(pdfFont, "", pdfFontRegular)
	pdf.AddUTF8FontFromBytes(pdfFont, "B", pdfFontBold)
	pdf.AddPage()

	drawFormHeader(pdf, inv)
	drawTableHeader(pdf)
	for _, line := range inv.Lines {
		drawItemRow(pdf, line)
	}
	drawTotalsRow(pdf, inv)
	pdf.Ln(3)

	drawSummary(pdf, inv)
	pdf.Ln(6)
	drawSignatures(pdf, inv)

	if pdf.Err() {
		return nil, fmt.Errorf("render invoice pdf: %w", pdf.Error())
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("output invoice pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func drawFormHeader(pdf *fpdf.Fpdf, inv *Invoice) {
	pageW, _ := pdf.GetPageSize()
	usable := pageW - pdfLeft - pdfRight

	pdf.SetFont(pdfFont, "", 7)
	pdf.SetX(pdfLeft + usable*0.55)
	pdf.MultiCell(usable*0.45, 3.2,
		"Приложение 26\nк приказу Министра финансов Республики Казахстан\nот 20 декабря 2012 года № 562", "", "R", false)
	pdf.SetX(pdfLeft + usable*0.55)
	pdf.CellFormat(usable*0.45, 3.6, "Форма З-2", "", 1, "R", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont(pdfFont, "", 9)
	pdf.CellFormat(usable*0.72, 5, "Организация (ИП): "+inv.SellerName, "", 0, "L", false, 0, "")
	pdf.CellFormat(usable*0.28, 5, "ИИН/БИН: "+inv.SellerBIN, "", 1, "R", false, 0, "")

	pdf.CellFormat(usable, 5,
		fmt.Sprintf("Номер документа: %s          Дата составления: %s",
			inv.DocumentNumber, inv.DocumentDate.Format("02.01.2006")), "", 1, "L", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont(pdfFont, "B", 11)
	pdf.CellFormat(usable, 6, "НАКЛАДНАЯ НА ОТПУСК ЗАПАСОВ НА СТОРОНУ", "", 1, "C", false, 0, "")
	pdf.Ln(1)

	drawParties(pdf, inv)
}

func drawParties(pdf *fpdf.Fpdf, inv *Invoice) {
	pageW, _ := pdf.GetPageSize()
	usable := pageW - pdfLeft - pdfRight
	pdf.SetFont(pdfFont, "", 9)
	line := func(label, value string) {
		pdf.SetFont(pdfFont, "B", 9)
		lw := pdf.GetStringWidth(label) + 2
		pdf.CellFormat(lw, 4.6, label, "", 0, "L", false, 0, "")
		pdf.SetFont(pdfFont, "", 9)
		pdf.MultiCell(usable-lw, 4.6, value, "", "L", false)
	}
	line("Отправитель:", inv.SellerName+"   (БИН "+inv.SellerBIN+")")
	line("Получатель:", inv.RecipientName)
	line("Адрес:", inv.RecipientAddress)
	if inv.ResponsibleForSupply != "" {
		line("Ответственный за поставку:", inv.ResponsibleForSupply)
	}
	pdf.Ln(1)
}

func drawTableHeader(pdf *fpdf.Fpdf) {
	pdf.SetFont(pdfFont, "B", 6)
	pdf.SetFillColor(235, 235, 235)
	x := pdf.GetX()
	y := pdf.GetY()
	const (
		hh     = 9.0
		headLH = 3.0
	)
	for _, c := range tableCols {
		lines := max(len(pdf.SplitText(c.head, c.width-1)), 1)
		pad := (hh - float64(lines)*headLH) / 2 // vertically center the header text
		pdf.Rect(x, y, c.width, hh, "DF")
		pdf.SetXY(x, y+pad)
		pdf.MultiCell(c.width, headLH, c.head, "", "C", false)
		x += c.width
	}
	pdf.SetXY(pdfLeft, y+hh)
}

func drawItemRow(pdf *fpdf.Fpdf, line Line) {
	pdf.SetFont(pdfFont, "", 7.5)
	texts := []string{
		fmt.Sprintf("%d", line.Index),
		line.Name,
		fmt.Sprintf("%d", line.Index),
		line.Unit,
		fmt.Sprintf("%d", line.Quantity),
		fmt.Sprintf("%d", line.Quantity),
		formatTenge(line.UnitPrice),
		formatTenge(line.Sum),
		formatTenge(line.VAT),
	}
	drawGridRow(pdf, texts)
}

func drawTotalsRow(pdf *fpdf.Fpdf, inv *Invoice) {
	pdf.SetFont(pdfFont, "B", 7.5)
	texts := []string{
		"", "Итого", "", "",
		fmt.Sprintf("%d", inv.TotalQuantity),
		fmt.Sprintf("%d", inv.TotalQuantity),
		"х",
		formatTenge(inv.TotalSum),
		formatTenge(inv.TotalVAT),
	}
	drawGridRow(pdf, texts)
}

// drawGridRow lays out one bordered table row, wrapping the «Наименование» column
// and sizing the row to the tallest cell.
func drawGridRow(pdf *fpdf.Fpdf, texts []string) {
	wrapLines := pdf.SplitText(texts[pdfTableWrap], tableCols[pdfTableWrap].width-1)
	rows := max(len(wrapLines), 1)
	h := pdfLineH * float64(rows)

	// Manual page break: keep the row on one page.
	_, pageH := pdf.GetPageSize()
	if pdf.GetY()+h > pageH-pdfBottomMargin {
		pdf.AddPage()
		drawTableHeader(pdf)
	}

	x := pdf.GetX()
	y := pdf.GetY()
	for i, c := range tableCols {
		pdf.Rect(x, y, c.width, h, "D")
		if i == pdfTableWrap {
			pdf.SetXY(x+0.5, y+0.3)
			pdf.MultiCell(c.width-1, pdfLineH, texts[i], "", c.align, false)
		} else {
			pdf.SetXY(x, y)
			pdf.CellFormat(c.width, h, texts[i], "", 0, c.align+"M", false, 0, "")
		}
		x += c.width
	}
	pdf.SetXY(pdfLeft, y+h)
}

func drawSummary(pdf *fpdf.Fpdf, inv *Invoice) {
	pageW, _ := pdf.GetPageSize()
	usable := pageW - pdfLeft - pdfRight
	pdf.SetFont(pdfFont, "", 9)
	pdf.CellFormat(usable, 5,
		fmt.Sprintf("Всего отпущено количество запасов: %d", inv.TotalQuantity), "", 1, "L", false, 0, "")
	pdf.SetFont(pdfFont, "B", 9)
	lbl := "На сумму (прописью): "
	lw := pdf.GetStringWidth(lbl) + 1
	pdf.CellFormat(lw, 5, lbl, "", 0, "L", false, 0, "")
	pdf.SetFont(pdfFont, "", 9)
	pdf.MultiCell(usable-lw, 5, capitalizeFirst(inv.AmountInWords), "", "L", false)
}

func drawSignatures(pdf *fpdf.Fpdf, inv *Invoice) {
	pdf.SetFont(pdfFont, "", 9)
	row := func(label, value string) {
		pdf.CellFormat(45, 6, label, "", 0, "L", false, 0, "")
		pdf.CellFormat(50, 6, "______________", "", 0, "C", false, 0, "")
		pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
	}
	row("Отпуск разрешил:", inv.Director)
	row("Главный бухгалтер:", inv.Accountant)
	row("Отпустил:", inv.ResponsibleForSupply)
	pdf.Ln(2)
	pdf.CellFormat(0, 6, "М.П.", "", 1, "L", false, 0, "")
}

func capitalizeFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	// Cyrillic upper-casing of the first rune.
	switch {
	case r[0] >= 'а' && r[0] <= 'я':
		r[0] = r[0] - ('а' - 'А')
	case r[0] >= 'a' && r[0] <= 'z':
		r[0] = r[0] - ('a' - 'A')
	}
	return string(r)
}
