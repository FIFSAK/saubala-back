package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// The workbook layout (one block per customer contract, repeated down the sheet):
//
//	header row : A = short name + №BIN/26NNNN/00 + date, one of B..F = legal name, one = address
//	item rows  : A = official tender name, B = ЕНС ТРУ code, C = unit, D = product name,
//	             E/F/G = plan qty/price/sum, then repeating (qty, price, sum) triplets —
//	             one triplet per delivery. A triplet whose column is labelled «остаток»
//	             in the header row is the undelivered remainder, not a delivery.
//	totals row : A empty, only sums — ignored.
//
// One known deviation: in a few rows qty and price are written in swapped order;
// they are detected by magnitude (price-like value in the qty column) and fixed.

var (
	contractNumRe = regexp.MustCompile(`№\s*(\d{12}/\d{6}/\d{2})`)
	binRe         = regexp.MustCompile(`БИН\s*:?\s*(\d{12})`)
	dateDotRe     = regexp.MustCompile(`(\d{2})[.,](\d{2})[.,](\d{4})`)
	dateISORe     = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
	spaceRe       = regexp.MustCompile(`\s+`)
)

// addressMarkers identify the header cell that carries the delivery address.
var addressMarkers = []string{"область", "облыс", "г.", "с.", "улица", "район", "проспект", "мкр", "микрорайон", "көш", "даңғ"}

type parsedLine struct {
	OfficialName string // column A — tender item name
	ProductName  string // column D, falling back to A — the real product
	Unit         string // column C
	PlanQty      int    // contract plan, units
	PriceTiyn    int64  // contract price per unit
	Deliveries   []int  // delivered qty per batch index (0 = no delivery in that batch)
	SwapFixed    bool   // qty/price arrived swapped and were corrected
}

func (l *parsedLine) delivered() int {
	total := 0
	for _, d := range l.Deliveries {
		total += d
	}
	return total
}

type parsedContract struct {
	Sheet     string
	Row       int
	Number    string
	Name      string // short customer name, e.g. «Алматы 15ГП»
	Address   string
	BIN       string
	Date      time.Time
	DateFound bool
	Lines     []*parsedLine
}

type parseReport struct {
	Warnings []string
}

func (r *parseReport) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// importSheets returns the sheet names to import: everything from the first
// sheet up to and including «Тараз, Кордай» (the customer asked to skip the
// trailing analytical sheets: Онко, шымкент узо, Балгабек).
func importSheets(f *excelize.File) ([]string, error) {
	all := f.GetSheetList()
	for i, name := range all {
		if strings.HasPrefix(strings.TrimSpace(name), "Тараз") {
			return all[:i+1], nil
		}
	}
	return nil, fmt.Errorf("лист «Тараз, Кордай» не найден: %v", all)
}

func parseWorkbook(path string) ([]*parsedContract, *parseReport, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("открыть %s: %w", path, err)
	}
	defer f.Close()

	sheets, err := importSheets(f)
	if err != nil {
		return nil, nil, err
	}

	report := &parseReport{}
	var contracts []*parsedContract
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, nil, fmt.Errorf("лист %q: %w", sheet, err)
		}
		contracts = append(contracts, parseSheet(sheet, rows, report)...)
	}
	return mergeByNumber(contracts, report), report, nil
}

func parseSheet(sheet string, rows [][]string, report *parseReport) []*parsedContract {
	var contracts []*parsedContract
	var cur *parsedContract
	remainderCol := -1 // 0-based column of the «остаток» triplet in the current block

	for i, row := range rows {
		rowNum := i + 1
		first := strings.TrimSpace(cell(row, 0))

		if m := contractNumRe.FindStringSubmatch(strings.Join(row, " ")); m != nil && first != "" && !isItemRow(row) {
			cur = parseHeader(sheet, rowNum, row, m[1], report)
			contracts = append(contracts, cur)
			remainderCol = -1
			for j, v := range row {
				if strings.Contains(strings.ToLower(v), "остаток") {
					remainderCol = j
				}
			}
			continue
		}
		if cur == nil || first == "" || !isItemRow(row) {
			continue
		}
		if line := parseItem(sheet, rowNum, row, remainderCol, report); line != nil {
			cur.Lines = append(cur.Lines, line)
		}
	}

	var out []*parsedContract
	for _, c := range contracts {
		if len(c.Lines) == 0 {
			report.warnf("%s:%d договор %s без строк — пропущен", c.Sheet, c.Row, c.Number)
			continue
		}
		out = append(out, c)
	}
	return out
}

// isItemRow reports whether columns E and F hold the numeric qty/price pair.
func isItemRow(row []string) bool {
	_, okQ := num(cell(row, 4))
	_, okP := num(cell(row, 5))
	return okQ && okP
}

func parseHeader(sheet string, rowNum int, row []string, number string, report *parseReport) *parsedContract {
	c := &parsedContract{Sheet: sheet, Row: rowNum, Number: number}
	head := strings.Join(row, " ")

	// Short customer name: column A text before the parenthesis / contract №.
	name := cell(row, 0)
	if idx := strings.IndexAny(name, "(№"); idx > 0 {
		name = name[:idx]
	}
	c.Name = clean(name)
	if c.Name == "" {
		c.Name = "Договор " + number
	}

	if m := binRe.FindStringSubmatch(head); m != nil {
		c.BIN = m[1]
	} else {
		c.BIN = number[:12] // the first segment of the lot number is the customer BIN
	}

	if d, ok := findDate(head); ok {
		c.Date, c.DateFound = d, true
	} else {
		c.Date = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		report.warnf("%s:%d договор %s: дата не распознана, взята 01.01.2026", sheet, rowNum, number)
	}

	for _, v := range row[1:] {
		lower := strings.ToLower(v)
		for _, marker := range addressMarkers {
			if strings.Contains(lower, marker) {
				c.Address = clean(v)
				break
			}
		}
		if c.Address != "" {
			break
		}
	}
	if c.Address == "" {
		c.Address = "не указан"
		report.warnf("%s:%d договор %s: адрес не найден", sheet, rowNum, number)
	}
	return c
}

func parseItem(sheet string, rowNum int, row []string, remainderCol int, report *parseReport) *parsedLine {
	qty, _ := num(cell(row, 4))
	price, _ := num(cell(row, 5))

	// A few rows have qty and price in swapped order: a per-unit price never
	// lands below 200 ₸ in this workbook while quantities that large (≥500)
	// only occur alongside four-digit prices.
	swapped := qty >= 500 && price <= 200
	if swapped {
		qty, price = price, qty
	}

	line := &parsedLine{
		OfficialName: clean(cell(row, 0)),
		Unit:         clean(cell(row, 2)),
		SwapFixed:    swapped,
	}
	line.ProductName = clean(cell(row, 3))
	if line.ProductName == "" {
		line.ProductName = line.OfficialName
	}

	planQty, ok := toInt(qty)
	if !ok {
		report.warnf("%s:%d дробное количество %v — округлено", sheet, rowNum, qty)
		planQty = int(math.Round(qty))
	}
	if planQty <= 0 {
		report.warnf("%s:%d нулевое количество по «%s» — строка пропущена", sheet, rowNum, line.ProductName)
		return nil
	}
	line.PlanQty = planQty
	line.PriceTiyn = tiyn(price)

	for col := 7; col < len(row); col += 3 {
		if remainderCol >= 0 && col >= remainderCol {
			break
		}
		dq, ok := num(cell(row, col))
		if !ok || dq <= 0 {
			line.Deliveries = append(line.Deliveries, 0)
			continue
		}
		dp, _ := num(cell(row, col+1))
		if dq >= 500 && dp > 0 && dp <= 200 {
			dq = dp // same swapped-pair defect in a delivery triplet
		}
		d, ok := toInt(dq)
		if !ok {
			report.warnf("%s:%d дробное количество поставки %v — округлено", sheet, rowNum, dq)
			d = int(math.Round(dq))
		}
		line.Deliveries = append(line.Deliveries, d)
	}
	return line
}

// mergeByNumber folds duplicate blocks of the same contract number (the
// workbook repeats two contracts) into one contract, appending lines.
func mergeByNumber(contracts []*parsedContract, report *parseReport) []*parsedContract {
	byNumber := map[string]*parsedContract{}
	var out []*parsedContract
	for _, c := range contracts {
		if prev, ok := byNumber[c.Number]; ok {
			report.warnf("договор %s встречается повторно (%s:%d) — строки объединены с %s:%d",
				c.Number, c.Sheet, c.Row, prev.Sheet, prev.Row)
			prev.Lines = append(prev.Lines, c.Lines...)
			continue
		}
		byNumber[c.Number] = c
		out = append(out, c)
	}
	return out
}

func cell(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

func num(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func toInt(v float64) (int, bool) {
	r := math.Round(v)
	return int(r), math.Abs(v-r) < 0.01
}

// tiyn converts a tenge amount to int64 tiyn (1 ₸ = 100 тиын).
func tiyn(tenge float64) int64 { return int64(math.Round(tenge * 100)) }

func clean(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func findDate(s string) (time.Time, bool) {
	if m := dateDotRe.FindStringSubmatch(s); m != nil {
		if d, ok := makeDate(m[3], m[2], m[1]); ok {
			return d, true
		}
	}
	if m := dateISORe.FindStringSubmatch(s); m != nil {
		if d, ok := makeDate(m[1], m[2], m[3]); ok {
			return d, true
		}
	}
	return time.Time{}, false
}

func makeDate(y, mo, d string) (time.Time, bool) {
	yy, _ := strconv.Atoi(y)
	mm, _ := strconv.Atoi(mo)
	dd, _ := strconv.Atoi(d)
	if yy < 2020 || yy > 2030 || mm < 1 || mm > 12 || dd < 1 || dd > 31 {
		return time.Time{}, false
	}
	return time.Date(yy, time.Month(mm), dd, 0, 0, 0, 0, time.UTC), true
}
