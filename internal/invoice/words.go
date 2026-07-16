package invoice

import "strings"

// Russian number-to-words for the «сумма прописью» field of the waybill.
//
// Gender matters: the thousands group is feminine (одна тысяча, две тысячи), the
// units group is masculine (один тенге, два тенге). тенге and тиын are treated as
// indeclinable, matching the customer's documents.

var (
	onesM    = [10]string{"", "один", "два", "три", "четыре", "пять", "шесть", "семь", "восемь", "девять"}
	onesF    = [10]string{"", "одна", "две", "три", "четыре", "пять", "шесть", "семь", "восемь", "девять"}
	teens    = [10]string{"десять", "одиннадцать", "двенадцать", "тринадцать", "четырнадцать", "пятнадцать", "шестнадцать", "семнадцать", "восемнадцать", "девятнадцать"}
	tensW    = [10]string{"", "", "двадцать", "тридцать", "сорок", "пятьдесят", "шестьдесят", "семьдесят", "восемьдесят", "девяносто"}
	hundreds = [10]string{"", "сто", "двести", "триста", "четыреста", "пятьсот", "шестьсот", "семьсот", "восемьсот", "девятьсот"}
)

// pluralForms holds the [one, few, many] forms of a scale word, e.g.
// {"тысяча","тысячи","тысяч"}.
type pluralForms [3]string

// pluralize picks the correct form of a scale word for n (Russian rules).
func pluralize(n int, f pluralForms) string {
	m100 := n % 100
	if m100 >= 11 && m100 <= 14 {
		return f[2]
	}
	switch n % 10 {
	case 1:
		return f[0]
	case 2, 3, 4:
		return f[1]
	default:
		return f[2]
	}
}

// oneWord returns the gendered word for a single unit digit.
func oneWord(o int, feminine bool) string {
	if feminine {
		return onesF[o]
	}
	return onesM[o]
}

// groupWords spells a 0..999 group, using the feminine unit forms when required.
func groupWords(num int, feminine bool) []string {
	h := num / 100
	t := (num / 10) % 10
	o := num % 10
	var parts []string
	if h > 0 {
		parts = append(parts, hundreds[h])
	}
	switch {
	case t > 1:
		parts = append(parts, tensW[t])
		if o > 0 {
			parts = append(parts, oneWord(o, feminine))
		}
	case t == 1:
		parts = append(parts, teens[o])
	default:
		if o > 0 {
			parts = append(parts, oneWord(o, feminine))
		}
	}
	return parts
}

// intToWords spells a non-negative integer in Russian. The units group is
// masculine; the thousands group is feminine.
func intToWords(n int64) string {
	if n == 0 {
		return "ноль"
	}
	// Split into base-1000 groups, least significant first.
	var groups []int
	for n > 0 {
		groups = append(groups, int(n%1000))
		n /= 1000
	}
	var words []string
	for i := len(groups) - 1; i >= 0; i-- {
		num := groups[i]
		if num == 0 {
			continue
		}
		words = append(words, groupWords(num, i == 1)...)
		switch i {
		case 1:
			words = append(words, pluralize(num, pluralForms{"тысяча", "тысячи", "тысяч"}))
		case 2:
			words = append(words, pluralize(num, pluralForms{"миллион", "миллиона", "миллионов"}))
		case 3:
			words = append(words, pluralize(num, pluralForms{"миллиард", "миллиарда", "миллиардов"}))
		}
	}
	return strings.Join(words, " ")
}

// TengeInWords renders a monetary amount (in tiyn) as the «сумма прописью» text,
// e.g. 33872000 -> "триста тридцать восемь тысяч семьсот двадцать тенге ноль тиын".
func TengeInWords(tiyn int64) string {
	if tiyn < 0 {
		tiyn = -tiyn
	}
	tenge := tiyn / 100
	kop := tiyn % 100
	return intToWords(tenge) + " тенге " + intToWords(kop) + " тиын"
}
