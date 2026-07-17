package invoice

import "testing"

func TestBuildVATInclusiveMatchesSample(t *testing.T) {
	// The three lines from the customer's sample invoice (prices in tiyn,
	// VAT-inclusive at 16%). Expected sums/VAT are taken from that file.
	inv := Build(BuildInput{
		VATRatePercent: 16,
		Lines: []LineInput{
			{Name: "Заменитель яиц", Unit: "штука", Quantity: 5, UnitPrice: 928000},
			{Name: "Чипсы", Unit: "штука", Quantity: 50, UnitPrice: 440800},
			{Name: "Мармелад", Unit: "штука", Quantity: 20, UnitPrice: 359600},
		},
	})

	wantSum := []int64{4640000, 22040000, 7192000}
	wantVAT := []int64{640000, 3040000, 992000}
	for i, l := range inv.Lines {
		if l.Sum != wantSum[i] {
			t.Errorf("line %d sum = %d, want %d", i, l.Sum, wantSum[i])
		}
		if l.VAT != wantVAT[i] {
			t.Errorf("line %d vat = %d, want %d", i, l.VAT, wantVAT[i])
		}
	}
	if inv.TotalQuantity != 75 {
		t.Errorf("total qty = %d, want 75", inv.TotalQuantity)
	}
	if inv.TotalSum != 33872000 {
		t.Errorf("total sum = %d, want 33872000", inv.TotalSum)
	}
	if inv.TotalVAT != 4672000 {
		t.Errorf("total vat = %d, want 4672000", inv.TotalVAT)
	}
	if inv.AmountInWords != "триста тридцать восемь тысяч семьсот двадцать тенге ноль тиын" {
		t.Errorf("amount in words = %q", inv.AmountInWords)
	}
}

func TestVATInclusiveZeroRate(t *testing.T) {
	if got := vatInclusive(1000, 0); got != 0 {
		t.Errorf("vatInclusive rate 0 = %d, want 0", got)
	}
}

func TestFormatTenge(t *testing.T) {
	cases := map[int64]string{
		33872000: "338 720.00",
		92800:    "928.00",
		100:      "1.00",
		0:        "0.00",
	}
	for tiyn, want := range cases {
		if got := formatTenge(tiyn); got != want {
			t.Errorf("formatTenge(%d) = %q, want %q", tiyn, got, want)
		}
	}
}
