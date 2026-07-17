package position

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/adjustment"
	"github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	domain "github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/internal/domain/supplier"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// --- in-memory fakes (only the methods the quantity-edit path touches carry
// behaviour; the rest are stubs to satisfy the domain Repository interfaces). ---

type fakePositions struct{ m map[string]*domain.Position }

func (f *fakePositions) GetByID(_ context.Context, id string) (*domain.Position, error) {
	p, ok := f.m[id]
	if !ok {
		return nil, store.ErrorNotFound
	}
	cp := *p
	return &cp, nil
}

// Update mirrors the Mongo adapter: it must NOT change stored quantity.
func (f *fakePositions) Update(_ context.Context, p *domain.Position) error {
	existing, ok := f.m[p.ID]
	if !ok {
		return store.ErrorNotFound
	}
	cp := *p
	cp.Quantity = existing.Quantity
	f.m[p.ID] = &cp
	return nil
}

func (f *fakePositions) IncrementQuantity(_ context.Context, id string, delta int) error {
	p, ok := f.m[id]
	if !ok {
		return store.ErrorNotFound
	}
	p.Quantity += delta
	return nil
}

func (f *fakePositions) DecrementIfAvailable(_ context.Context, id string, qty int) (bool, error) {
	p, ok := f.m[id]
	if !ok {
		return false, store.ErrorNotFound
	}
	if p.Quantity < qty {
		return false, nil
	}
	p.Quantity -= qty
	return true, nil
}

func (f *fakePositions) Create(context.Context, *domain.Position) error { return nil }
func (f *fakePositions) GetByIDs(context.Context, []string) ([]domain.Position, error) {
	return nil, nil
}
func (f *fakePositions) Delete(context.Context, string) error { return nil }
func (f *fakePositions) List(context.Context, domain.Filter) ([]domain.Position, int64, error) {
	return nil, 0, nil
}
func (f *fakePositions) CountByBrand(context.Context, string) (int64, error)    { return 0, nil }
func (f *fakePositions) CountBySupplier(context.Context, string) (int64, error) { return 0, nil }

type fakeAdjustments struct {
	items      []adjustment.Adjustment
	failCreate bool
}

func (f *fakeAdjustments) Create(_ context.Context, a *adjustment.Adjustment) error {
	if f.failCreate {
		return errors.New("ledger write failed")
	}
	f.items = append(f.items, *a)
	return nil
}

func (f *fakeAdjustments) ListByPosition(_ context.Context, positionID string) ([]adjustment.Adjustment, error) {
	var out []adjustment.Adjustment
	for _, a := range f.items {
		if a.PositionID == positionID {
			out = append(out, a)
		}
	}
	return out, nil
}

type fakeReceipts struct{ items []receipt.Receipt }

func (f *fakeReceipts) Create(context.Context, *receipt.Receipt) error { return nil }
func (f *fakeReceipts) GetByID(context.Context, string) (*receipt.Receipt, error) {
	return nil, store.ErrorNotFound
}
func (f *fakeReceipts) List(context.Context, receipt.Filter) ([]receipt.Receipt, int64, error) {
	return nil, 0, nil
}
func (f *fakeReceipts) CountBySupplier(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeReceipts) InvoiceTotalBySupplier(context.Context, []string) (map[string]int64, error) {
	return nil, nil
}
func (f *fakeReceipts) ListByPosition(context.Context, string) ([]receipt.Receipt, error) {
	return f.items, nil
}

type fakeReleases struct{ items []release.Release }

func (f *fakeReleases) Create(context.Context, *release.Release) error { return nil }
func (f *fakeReleases) GetByID(context.Context, string) (*release.Release, error) {
	return nil, store.ErrorNotFound
}
func (f *fakeReleases) List(context.Context, release.Filter) ([]release.Release, int64, error) {
	return nil, 0, nil
}
func (f *fakeReleases) ListByPosition(context.Context, string) ([]release.Release, error) {
	return f.items, nil
}
func (f *fakeReleases) UpdateWaybill(context.Context, string, release.WaybillUpdate) error {
	return nil
}
func (f *fakeReleases) CountByContract(context.Context, string) (int64, error) { return 0, nil }
func (f *fakeReleases) ReleasedByContract(context.Context, string) (map[string]int, error) {
	return nil, nil
}
func (f *fakeReleases) ReleasedByContracts(context.Context, []string) (map[string]map[string]int, error) {
	return nil, nil
}
func (f *fakeReleases) CountByOrganization(context.Context, string) (int64, error) { return 0, nil }

type fakeBrands struct{}

func (fakeBrands) Create(context.Context, *brand.Brand) error { return nil }
func (fakeBrands) GetByID(_ context.Context, id string) (*brand.Brand, error) {
	return &brand.Brand{ID: id}, nil
}
func (fakeBrands) GetByIDs(context.Context, []string) ([]brand.Brand, error) { return nil, nil }
func (fakeBrands) GetByName(context.Context, string) (*brand.Brand, error) {
	return nil, store.ErrorNotFound
}
func (fakeBrands) Update(context.Context, *brand.Brand) error { return nil }
func (fakeBrands) SoftDelete(context.Context, string) error   { return nil }
func (fakeBrands) List(context.Context, brand.Filter) ([]brand.Brand, int64, error) {
	return nil, 0, nil
}

type fakeSuppliers struct{}

func (fakeSuppliers) Create(context.Context, *supplier.Supplier) error { return nil }
func (fakeSuppliers) GetByID(context.Context, string) (*supplier.Supplier, error) {
	return &supplier.Supplier{}, nil
}
func (fakeSuppliers) GetByIDs(context.Context, []string) ([]supplier.Supplier, error) {
	return nil, nil
}
func (fakeSuppliers) Update(context.Context, *supplier.Supplier) error { return nil }
func (fakeSuppliers) Delete(context.Context, string) error             { return nil }
func (fakeSuppliers) List(context.Context, supplier.Filter) ([]supplier.Supplier, int64, error) {
	return nil, 0, nil
}

type fakeContracts struct{}

func (fakeContracts) Create(context.Context, *contract.Contract) error { return nil }
func (fakeContracts) GetByID(context.Context, string) (*contract.Contract, error) {
	return nil, store.ErrorNotFound
}
func (fakeContracts) GetByIDs(context.Context, []string) ([]contract.Contract, error) {
	return nil, nil
}
func (fakeContracts) GetByNumber(context.Context, string) (*contract.Contract, error) {
	return nil, store.ErrorNotFound
}
func (fakeContracts) Update(context.Context, *contract.Contract) error { return nil }
func (fakeContracts) Delete(context.Context, string) error             { return nil }
func (fakeContracts) List(context.Context, contract.Filter) ([]contract.Contract, int64, error) {
	return nil, 0, nil
}
func (fakeContracts) CountByPosition(context.Context, string) (int64, error) { return 0, nil }

// harness wires the service with a single seeded position at the given quantity.
type harness struct {
	svc *Service
	adj *fakeAdjustments
	pos *fakePositions
	rec *fakeReceipts
	id  string
}

func newHarness(startQty int) *harness {
	id := "pos-1"
	pos := &fakePositions{m: map[string]*domain.Position{
		id: {ID: id, Name: "Товар", BrandID: "b1", Quantity: startQty},
	}}
	adj := &fakeAdjustments{}
	rec := &fakeReceipts{}
	rel := &fakeReleases{}
	svc := NewService(pos, fakeBrands{}, fakeSuppliers{}, rec, rel, fakeContracts{}, adj)
	return &harness{svc: svc, adj: adj, pos: pos, rec: rec, id: id}
}

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }

func TestUpdate_QuantityIncrease_RecordsPositiveAdjustment(t *testing.T) {
	h := newHarness(10)

	p, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(25), ActorID: "u1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Quantity != 25 {
		t.Fatalf("returned quantity = %d, want 25", p.Quantity)
	}
	if got := h.pos.m[h.id].Quantity; got != 25 {
		t.Fatalf("stored quantity = %d, want 25", got)
	}
	if len(h.adj.items) != 1 {
		t.Fatalf("adjustments recorded = %d, want 1", len(h.adj.items))
	}
	a := h.adj.items[0]
	if a.Delta != 15 {
		t.Errorf("adjustment delta = %d, want +15", a.Delta)
	}
	if a.Note != "корректировка" {
		t.Errorf("adjustment note = %q, want %q", a.Note, "корректировка")
	}
	if a.CreatedBy != "u1" {
		t.Errorf("adjustment createdBy = %q, want u1", a.CreatedBy)
	}
}

func TestUpdate_QuantityDecrease_RecordsNegativeAdjustment(t *testing.T) {
	h := newHarness(10)

	p, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(4)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Quantity != 4 || h.pos.m[h.id].Quantity != 4 {
		t.Fatalf("quantity = %d/%d, want 4", p.Quantity, h.pos.m[h.id].Quantity)
	}
	if len(h.adj.items) != 1 || h.adj.items[0].Delta != -6 {
		t.Fatalf("want one -6 adjustment, got %+v", h.adj.items)
	}
}

func TestUpdate_QuantityUnchanged_RecordsNothing(t *testing.T) {
	h := newHarness(10)

	if _, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(10)}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.adj.items) != 0 {
		t.Fatalf("no-op edit recorded %d adjustments, want 0", len(h.adj.items))
	}
	if h.pos.m[h.id].Quantity != 10 {
		t.Fatalf("quantity changed to %d, want 10", h.pos.m[h.id].Quantity)
	}
}

func TestUpdate_NegativeQuantity_Rejected(t *testing.T) {
	h := newHarness(10)

	_, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(-1)})
	var werr *web.Error
	if !errors.As(err, &werr) || werr.Status != 400 {
		t.Fatalf("err = %v, want 400 web.Error", err)
	}
	if len(h.adj.items) != 0 || h.pos.m[h.id].Quantity != 10 {
		t.Fatalf("rejected edit still mutated state: adj=%d qty=%d", len(h.adj.items), h.pos.m[h.id].Quantity)
	}
}

func TestUpdate_DescriptiveOnly_LeavesStockAndLedger(t *testing.T) {
	h := newHarness(10)

	p, err := h.svc.Update(context.Background(), h.id, UpdateInput{Name: ptrStr("Новое имя")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Новое имя" {
		t.Errorf("name = %q, want updated", p.Name)
	}
	if len(h.adj.items) != 0 || h.pos.m[h.id].Quantity != 10 {
		t.Fatalf("descriptive edit touched stock/ledger: adj=%d qty=%d", len(h.adj.items), h.pos.m[h.id].Quantity)
	}
}

func TestUpdate_LedgerFailure_RollsBackStock(t *testing.T) {
	// Increase path: +15 applied, ledger write fails -> stock rolled back to 10.
	h := newHarness(10)
	h.adj.failCreate = true
	if _, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(25)}); err == nil {
		t.Fatal("expected error when ledger write fails")
	}
	if got := h.pos.m[h.id].Quantity; got != 10 {
		t.Fatalf("stock not rolled back on increase: quantity = %d, want 10", got)
	}

	// Decrease path: -6 drawn, ledger write fails -> stock restored to 10.
	h2 := newHarness(10)
	h2.adj.failCreate = true
	if _, err := h2.svc.Update(context.Background(), h2.id, UpdateInput{Quantity: ptrInt(4)}); err == nil {
		t.Fatal("expected error when ledger write fails")
	}
	if got := h2.pos.m[h2.id].Quantity; got != 10 {
		t.Fatalf("stock not rolled back on decrease: quantity = %d, want 10", got)
	}
}

func TestMovements_ReconcilesReceiptsAndAdjustments(t *testing.T) {
	h := newHarness(10)
	// Simulate the opening-balance receipt that seeded the 10 units.
	h.rec.items = []receipt.Receipt{{
		ID:    "r1",
		Date:  time.Now().Add(-time.Hour).UTC(),
		Note:  "opening balance",
		Lines: []receipt.Line{{PositionID: h.id, Quantity: 10}},
	}}

	if _, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(25)}); err != nil {
		t.Fatalf("increase failed: %v", err)
	}
	if _, err := h.svc.Update(context.Background(), h.id, UpdateInput{Quantity: ptrInt(19)}); err != nil {
		t.Fatalf("decrease failed: %v", err)
	}

	movements, err := h.svc.Movements(context.Background(), h.id)
	if err != nil {
		t.Fatalf("movements error: %v", err)
	}

	var sum, adjCount int
	for _, m := range movements {
		sum += m.Quantity
		if m.Type == domain.MovementAdjustment {
			adjCount++
		}
	}
	if adjCount != 2 {
		t.Errorf("adjustment movements = %d, want 2 (+15, -6)", adjCount)
	}
	if sum != 19 {
		t.Errorf("movement sum = %d, want 19 (reconciles with final quantity)", sum)
	}
	if final := h.pos.m[h.id].Quantity; final != sum {
		t.Errorf("final quantity %d != movement sum %d", final, sum)
	}
}
