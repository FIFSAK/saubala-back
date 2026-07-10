// Command import-xlsx performs the one-off migration of the customer's
// «поставки 2026.xlsx» workbook into the application database.
//
// Mapping (agreed with the customer):
//   - each customer block on a sheet → Contract (number, date, BIN, address, plan lines);
//   - each unique product name → one Position with shared stock; lot/expiry are
//     synthetic placeholders (IMPORT-2026 / 31.12.2027), brand is parsed from the
//     product name, purchase price = first-seen contract price;
//   - opening stock of every position = its total delivered quantity, so after
//     importing all releases the remaining stock is zero;
//   - each delivery triplet → a Release against the contract, dated by the
//     contract date (real delivery dates are absent from the workbook).
//
// It reuses the service layer, so all stock rules apply. Trailing sheets after
// «Тараз, Кордай» (Онко, шымкент узо, Балгабек) are skipped by request.
//
// Usage:
//
//	go run ./cmd/import-xlsx -file "поставки 2026.xlsx" -dry-run   # parse + report only
//	go run ./cmd/import-xlsx -file "поставки 2026.xlsx"            # import into Mongo
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/FIFSAK/saubala-back/internal/config"
	"github.com/FIFSAK/saubala-back/internal/repository"
	"github.com/FIFSAK/saubala-back/internal/service"
	contractsvc "github.com/FIFSAK/saubala-back/internal/service/contract"
	positionsvc "github.com/FIFSAK/saubala-back/internal/service/position"
	releasesvc "github.com/FIFSAK/saubala-back/internal/service/release"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

const (
	importLot     = "IMPORT-2026"
	fallbackBrand = "Без бренда"
)

// brandRules map a canonical brand name to lower-case substrings of product
// names. First match wins; unmatched products go to the fallback brand.
var brandRules = []struct {
	Brand string
	Keys  []string
}{
	{"Nutricia", []string{"nutricia", "nutrilon", "нутрилон", "neocate", "малютка", "лопрофин", "loprofin", "milupa", "нутриция", "fortini", "нутризон", "nutrini"}},
	{"Nestlé", []string{"nestle", "nestl", "nestogen", "нестожен", "clinutren", "alfamino", "resource"}},
	{"Bezgluten", []string{"bezgluten"}},
	{"Balviten", []string{"balviten", "балвитен"}},
	{"Sofra", []string{"sofra", "софра", "anadolu", "tanem"}},
	{"Taranis", []string{"taranis", "таранис"}},
	{"Dr. Schär", []string{"schär", "schar", "салинис", "salinis", "curvies"}},
	{"Metax", []string{"metax", "метакс"}},
	{"Mevalia", []string{"mevalia", "мевалия", "mevali"}},
	{"МакМастер", []string{"макмастер", "mcmaster", "мак мастер"}},
	{"Арт Лайф", []string{"арт лайф", "артлайф", "art life", "арт-лайф"}},
	{"Comida", []string{"comida", "комида"}},
	{"Hammermühle", []string{"hammerm"}},
	{"Huber", []string{"huber"}},
	{"Wilmersburger", []string{"wilmersburger"}},
	{"Nutri Free", []string{"nutri free", "nutrifree"}},
	{"Gullon", []string{"gullon"}},
	{"Schlagfix", []string{"schlagfix"}},
	{"Okovital", []string{"okovital"}},
	{"Aproten", []string{"aproten", "апротен"}},
	{"Amino", []string{"amino ", "амино"}},
	{"Flavis", []string{"flavis", "флавис"}},
	{"Увелка", []string{"увелка"}},
	{"Vitaflo", []string{"vitaflo", "prozero", "pro zero"}},
	{"Caramba", []string{"caramba"}},
	{"БенАмин", []string{"бенамин"}},
	{"Similac", []string{"similac", "симилак"}},
	{"Humana", []string{"humana", "хумана"}},
	{"Hipp", []string{"hipp", "хипп"}},
	{"Кабрита", []string{"kabrita", "кабрита"}},
	{"Фрисо", []string{"friso", "фрисо"}},
}

func brandOf(productName string) string {
	lower := strings.ToLower(productName)
	for _, r := range brandRules {
		for _, k := range r.Keys {
			if strings.Contains(lower, k) {
				return r.Brand
			}
		}
	}
	return fallbackBrand
}

// product is the aggregate of one unique product name across every contract:
// it becomes one Position with shared stock.
type product struct {
	Name         string
	OfficialName string
	Brand        string
	PriceTiyn    int64 // first-seen contract price
	Delivered    int   // total across all deliveries → opening stock
	Planned      int
}

func collectProducts(contracts []*parsedContract) map[string]*product {
	products := map[string]*product{}
	for _, c := range contracts {
		for _, l := range c.Lines {
			key := strings.ToLower(l.ProductName)
			p, ok := products[key]
			if !ok {
				p = &product{
					Name:         l.ProductName,
					OfficialName: l.OfficialName,
					Brand:        brandOf(l.ProductName),
					PriceTiyn:    l.PriceTiyn,
				}
				products[key] = p
			}
			p.Planned += l.PlanQty
			p.Delivered += l.delivered()
		}
	}
	return products
}

func main() {
	log.SetFlags(0)
	file := flag.String("file", "поставки 2026.xlsx", "путь к файлу Excel")
	dryRun := flag.Bool("dry-run", false, "только разобрать файл и показать отчёт, без записи в базу")
	flag.Parse()

	if err := run(*file, *dryRun); err != nil {
		log.Fatalf("импорт не выполнен: %v", err)
	}
}

func run(path string, dryRun bool) error {
	contracts, report, err := parseWorkbook(path)
	if err != nil {
		return err
	}
	products := collectProducts(contracts)
	printSummary(contracts, products, report)

	if dryRun {
		log.Println("\n► dry-run: запись в базу не выполнялась")
		return nil
	}
	return importAll(contracts, products)
}

func printSummary(contracts []*parsedContract, products map[string]*product, report *parseReport) {
	lines, deliveries, releaseUnits := 0, 0, 0
	var planSum int64
	for _, c := range contracts {
		lines += len(c.Lines)
		for _, l := range c.Lines {
			planSum += l.PriceTiyn * int64(l.PlanQty)
			for _, d := range l.Deliveries {
				if d > 0 {
					deliveries++
					releaseUnits += d
				}
			}
		}
	}
	brands := map[string]int{}
	for _, p := range products {
		brands[p.Brand]++
	}

	log.Printf("► разобрано: договоров %d, строк %d, поставок %d (%d ед.), уникальных товаров %d, брендов %d",
		len(contracts), lines, deliveries, releaseUnits, len(products), len(brands))
	log.Printf("► сумма по договорам: %.2f ₸", float64(planSum)/100)

	if len(report.Warnings) > 0 {
		log.Printf("\n► предупреждения (%d):", len(report.Warnings))
		for _, w := range report.Warnings {
			log.Printf("  · %s", w)
		}
	}
}

func importAll(contracts []*parsedContract, products map[string]*product) error {
	ctx := context.Background()

	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	m, err := store.NewMongo(ctx, cfg.Mongo.URI, cfg.Mongo.DB)
	if err != nil {
		return fmt.Errorf("mongo connect: %w", err)
	}
	defer m.Close(ctx)

	repos, err := repository.New(repository.WithMongoStore(ctx, m))
	if err != nil {
		return fmt.Errorf("repositories: %w", err)
	}
	tm := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	svc, err := service.New(
		service.Dependencies{Repositories: repos, TokenManager: tm},
		service.WithUserService(),
		service.WithBrandService(),
		service.WithPositionService(),
		service.WithReceiptService(),
		service.WithContractService(),
		service.WithReleaseService(),
	)
	if err != nil {
		return fmt.Errorf("services: %w", err)
	}

	// Guard against a double import: every contract number must be new.
	for _, c := range contracts {
		if _, err := repos.Contract.GetByNumber(ctx, c.Number); err == nil {
			return fmt.Errorf("договор %s уже есть в базе — похоже, импорт уже выполнялся; повторный запуск задвоил бы остатки", c.Number)
		}
	}

	if err := svc.User.EnsureSuperAdmin(ctx, cfg.SuperAdmin.Email, cfg.SuperAdmin.Password); err != nil {
		return fmt.Errorf("super admin: %w", err)
	}
	admin, err := repos.User.GetByEmail(ctx, cfg.SuperAdmin.Email)
	if err != nil {
		return fmt.Errorf("load super admin: %w", err)
	}

	// --- brands ---
	brandIDs := map[string]string{}
	for _, p := range products {
		if _, ok := brandIDs[p.Brand]; ok {
			continue
		}
		if existing, err := repos.Brand.GetByName(ctx, p.Brand); err == nil {
			brandIDs[p.Brand] = existing.ID
			continue
		}
		b, err := svc.Brand.Create(ctx, p.Brand)
		if err != nil {
			return fmt.Errorf("бренд %q: %w", p.Brand, err)
		}
		brandIDs[p.Brand] = b.ID
	}
	log.Printf("► брендов: %d", len(brandIDs))

	// --- positions (opening stock = everything that was ever delivered) ---
	expiry := time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)
	keys := make([]string, 0, len(products))
	for k := range products {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	positionIDs := map[string]string{}
	for _, k := range keys {
		p := products[k]
		pos, err := svc.Position.Create(ctx, positionsvc.CreateInput{
			Name:          p.Name,
			BrandID:       brandIDs[p.Brand],
			ContractName:  p.OfficialName,
			ExpiryDate:    expiry,
			LotNumber:     importLot,
			PurchasePrice: p.PriceTiyn,
			Quantity:      p.Delivered,
			CreatedBy:     admin.ID,
		})
		if err != nil {
			return fmt.Errorf("позиция %q: %w", p.Name, err)
		}
		positionIDs[k] = pos.ID
	}
	log.Printf("► позиций: %d (открывающий остаток = отгруженному объёму)", len(positionIDs))

	// --- contracts + releases ---
	nContracts, nReleases := 0, 0
	for _, c := range contracts {
		inputs := make([]contractsvc.LineInput, len(c.Lines))
		for i, l := range c.Lines {
			plan := l.PlanQty
			if d := l.delivered(); d > plan {
				log.Printf("  · договор %s, «%s»: отгружено %d больше плана %d — план увеличен", c.Number, l.ProductName, d, plan)
				plan = d
			}
			price := l.PriceTiyn
			inputs[i] = contractsvc.LineInput{
				PositionID:      positionIDs[strings.ToLower(l.ProductName)],
				PlannedQuantity: plan,
				PlannedPrice:    &price,
			}
		}
		created, err := svc.Contract.Create(ctx, contractsvc.CreateInput{
			Name:            c.Name,
			CustomerAddress: c.Address,
			ContractNumber:  c.Number,
			ContractDate:    c.Date,
			BIN:             c.BIN,
			Lines:           inputs,
			CreatedBy:       admin.ID,
		})
		if err != nil {
			return fmt.Errorf("договор %s (%s:%d): %w", c.Number, c.Sheet, c.Row, err)
		}
		nContracts++

		// One release per delivery batch: batch k gathers column-triplet k of
		// every line. Real delivery dates are absent, so all releases carry the
		// contract date.
		maxBatches := 0
		for _, l := range c.Lines {
			if len(l.Deliveries) > maxBatches {
				maxBatches = len(l.Deliveries)
			}
		}
		for batch := 0; batch < maxBatches; batch++ {
			var lines []releasesvc.LineInput
			for i, l := range c.Lines {
				if batch >= len(l.Deliveries) || l.Deliveries[batch] <= 0 {
					continue
				}
				lines = append(lines, releasesvc.LineInput{
					ContractLineID: created.Lines[i].ID,
					PositionID:     positionIDs[strings.ToLower(l.ProductName)],
					Quantity:       l.Deliveries[batch],
				})
			}
			if len(lines) == 0 {
				continue
			}
			if _, err := svc.Release.Create(ctx, releasesvc.CreateInput{
				ContractID: created.ID,
				Date:       c.Date,
				Note:       fmt.Sprintf("Импорт из Excel: поставка %d", batch+1),
				Lines:      lines,
				CreatedBy:  admin.ID,
			}); err != nil {
				return fmt.Errorf("отгрузка %d по договору %s: %w", batch+1, c.Number, err)
			}
			nReleases++
		}
	}
	log.Printf("► договоров: %d, отгрузок: %d", nContracts, nReleases)
	log.Println("► импорт завершён")
	return nil
}
