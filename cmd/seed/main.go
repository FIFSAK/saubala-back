// Command seed populates the database for local development/demo:
// it ensures a super administrator with a simple password, creates a couple of
// extra users, and fills the warehouse with realistic brands, positions,
// receipts, contracts and releases. It reuses the service layer, so all the
// stock rules (opening receipts, atomic stock, plan control) are respected.
//
// Usage:
//
//	go run ./cmd/seed                 # seed if the database is empty
//	go run ./cmd/seed -reset          # drop business data first, then seed
//	go run ./cmd/seed -password 123456789
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/FIFSAK/saubala-back/internal/config"
	branddom "github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/repository"
	"github.com/FIFSAK/saubala-back/internal/service"
	contractsvc "github.com/FIFSAK/saubala-back/internal/service/contract"
	positionsvc "github.com/FIFSAK/saubala-back/internal/service/position"
	receiptsvc "github.com/FIFSAK/saubala-back/internal/service/receipt"
	releasesvc "github.com/FIFSAK/saubala-back/internal/service/release"
	usersvc "github.com/FIFSAK/saubala-back/internal/service/user"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

func main() {
	log.SetFlags(0)
	reset := flag.Bool("reset", false, "drop business collections before seeding")
	password := flag.String("password", "admin12345", "super administrator password")
	flag.Parse()

	if err := run(*reset, *password); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
}

// tg converts whole tenge to int64 tiyn (1 ₸ = 100 тиын).
func tg(tenge int64) int64 { return tenge * 100 }

// priceP returns a pointer to a tiyn price built from whole tenge.
func priceP(tenge int64) *int64 { v := tg(tenge); return &v }

type seeder struct {
	ctx   context.Context
	svc   *service.Services
	repos *repository.Repositories
	actor string
}

func (s *seeder) brand(name string) string {
	b, err := s.svc.Brand.Create(s.ctx, name)
	if err == nil {
		return b.ID
	}
	if existing, gerr := s.repos.Brand.GetByName(s.ctx, name); gerr == nil {
		return existing.ID
	}
	log.Printf("  ! бренд %q: %v", name, err)
	return ""
}

func (s *seeder) position(in positionsvc.CreateInput) string {
	in.CreatedBy = s.actor
	p, err := s.svc.Position.Create(s.ctx, in)
	if err != nil {
		log.Printf("  ! позиция %q: %v", in.Name, err)
		return ""
	}
	return p.ID
}

func run(reset bool, password string) error {
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

	if reset {
		for _, name := range []string{"users", "brands", "positions", "receipts", "contracts", "releases"} {
			if err := m.DB.Collection(name).Drop(ctx); err != nil {
				log.Printf("drop %s: %v", name, err)
			}
		}
		log.Println("► коллекции очищены (-reset)")
	}

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

	// --- users ---
	if err := svc.User.EnsureSuperAdmin(ctx, cfg.SuperAdmin.Email, password); err != nil {
		return fmt.Errorf("super admin: %w", err)
	}
	admin, err := repos.User.GetByEmail(ctx, cfg.SuperAdmin.Email)
	if err != nil {
		return fmt.Errorf("load super admin: %w", err)
	}
	log.Printf("► супер-админ: %s", admin.Email)

	for _, u := range []usersvc.CreateInput{
		{Email: "manager@saubala.kz", Password: "manager123", FullName: "Гульнара Сапарова", Role: "admin"},
		{Email: "sklad@saubala.kz", Password: "sklad12345", FullName: "Ержан Болатов", Role: "user"},
	} {
		if _, err := svc.User.Create(ctx, u); err != nil {
			log.Printf("  · пользователь %s: %v", u.Email, err)
		} else {
			log.Printf("  + пользователь %s (%s)", u.Email, u.Role)
		}
	}

	// Only seed business data into an empty database unless -reset was given.
	_, brandsTotal, _ := repos.Brand.List(ctx, branddom.Filter{Page: 1, PageSize: 1})
	if brandsTotal > 0 && !reset {
		log.Printf("► бизнес-данные уже есть (брендов: %d) — пропускаю. Используйте -reset для пересоздания.", brandsTotal)
		printCreds(admin.Email, password)
		return nil
	}

	now := time.Now().UTC()
	s := &seeder{ctx: ctx, svc: svc, repos: repos, actor: admin.ID}

	// --- brands ---
	bNutricia := s.brand("Nutricia")
	bNestle := s.brand("Nestlé Health Science")
	bAbbott := s.brand("Abbott")
	bHumana := s.brand("Humana")
	log.Println("► бренды созданы")

	// --- positions (with opening stock) ---
	pos := map[string]string{}
	pos["neocate"] = s.position(positionsvc.CreateInput{
		Name: "Neocate LCP, банка 400 г", BrandID: bNutricia,
		ContractName: "Смесь аминокислотная Neocate LCP", SupplierName: "Nutricia Kazakhstan",
		ExpiryDate: now.AddDate(1, 0, 0), LotNumber: "NEO-2026-014",
		PurchasePrice: tg(18500), Quantity: 120, MassGrams: 400,
	})
	pos["pepti"] = s.position(positionsvc.CreateInput{
		Name: "Nutrilon Пепти Гастро, 450 г", BrandID: bNutricia,
		ContractName: "Смесь Nutrilon Пепти Гастро", SupplierName: "Nutricia Kazakhstan",
		ExpiryDate: now.AddDate(0, 8, 0), LotNumber: "NTR-PG-0451",
		PurchasePrice: tg(9800), Quantity: 80, MassGrams: 450,
	})
	pos["alfamino"] = s.position(positionsvc.CreateInput{
		Name: "Alfamino, 400 г", BrandID: bNestle,
		ContractName: "Смесь аминокислотная Alfamino", SupplierName: "Nestlé Kazakhstan",
		ExpiryDate: now.AddDate(1, 1, 0), LotNumber: "ALF-400-22",
		PurchasePrice: tg(21000), Quantity: 60, MassGrams: 400,
	})
	pos["pediasure"] = s.position(positionsvc.CreateInput{
		Name: "PediaSure со вкусом ванили, 400 г", BrandID: bAbbott,
		ContractName: "Питание энтеральное PediaSure", SupplierName: "Abbott Logistics",
		ExpiryDate: now.AddDate(0, 5, 0), LotNumber: "PS-VAN-400",
		PurchasePrice: tg(6500), Quantity: 200, MassGrams: 400,
	})
	pos["similac"] = s.position(positionsvc.CreateInput{
		Name: "Similac Изомил, 375 г", BrandID: bAbbott,
		ContractName: "Смесь Similac Изомил (соевая)", SupplierName: "Abbott Logistics",
		ExpiryDate: now.AddDate(0, 0, 25), LotNumber: "SIM-ISO-375",
		PurchasePrice: tg(7200), Quantity: 40, MassGrams: 375,
	})
	pos["humana"] = s.position(positionsvc.CreateInput{
		Name: "Humana SL (соевая), 500 г", BrandID: bHumana,
		ContractName: "Смесь Humana SL", SupplierName: "Humana GmbH",
		ExpiryDate: now.AddDate(0, 0, 12), LotNumber: "HUM-SL-500",
		PurchasePrice: tg(5400), Quantity: 5, MassGrams: 500,
	})
	pos["nutrini"] = s.position(positionsvc.CreateInput{
		Name: "Nutrini Energy Multi Fibre, 200 мл", BrandID: bNutricia,
		ContractName: "Питание Nutrini Energy Multi Fibre", SupplierName: "Nutricia Kazakhstan",
		ExpiryDate: now.AddDate(1, 5, 0), LotNumber: "NUT-EMF-200",
		PurchasePrice: tg(1200), Quantity: 0, MassGrams: 200,
	})
	log.Println("► позиции созданы (с открывающими остатками)")

	// --- extra receipt (restock) ---
	if pos["pediasure"] != "" && pos["pepti"] != "" {
		if _, err := svc.Receipt.Create(ctx, receiptsvc.CreateInput{
			Date: now.AddDate(0, 0, -5), Supplier: "Abbott Logistics", DocumentNumber: "ТТН-3391",
			Note: "Плановая поставка", CreatedBy: admin.ID,
			Lines: []receiptsvc.LineInput{
				{PositionID: pos["pediasure"], Quantity: 100},
				{PositionID: pos["pepti"], Quantity: 40},
			},
		}); err != nil {
			log.Printf("  ! поступление: %v", err)
		} else {
			log.Println("► поступление оформлено (+140 шт)")
		}
	}

	// --- contracts ---
	c1, err := svc.Contract.Create(ctx, contractsvc.CreateInput{
		Name:            "Государственный закуп специализированного питания на 2026 год",
		CustomerAddress: "г. Астана, ул. Бейбітшілік, 11, ГКП «Центр социальной помощи»",
		ContractNumber:  "ГЗ-2026-014", ContractDate: now.AddDate(0, -1, 0), BIN: "123456789012",
		CreatedBy: admin.ID,
		Lines: []contractsvc.LineInput{
			{PositionID: pos["neocate"], PlannedQuantity: 50, PlannedPrice: priceP(19500)},
			{PositionID: pos["pediasure"], PlannedQuantity: 120, PlannedPrice: priceP(7000)},
		},
	})
	if err != nil {
		log.Printf("  ! договор ГЗ-2026-014: %v", err)
	}
	c2, err := svc.Contract.Create(ctx, contractsvc.CreateInput{
		Name:            "Договор поставки лечебного питания № 77",
		CustomerAddress: "г. Алматы, пр. Абая, 150, КГУ «Детский реабилитационный центр»",
		ContractNumber:  "ДП-77/2026", ContractDate: now.AddDate(0, -2, 0), BIN: "210987654321",
		CreatedBy: admin.ID,
		Lines: []contractsvc.LineInput{
			{PositionID: pos["alfamino"], PlannedQuantity: 30, PlannedPrice: priceP(22500)},
			{PositionID: pos["pepti"], PlannedQuantity: 40},
		},
	})
	if err != nil {
		log.Printf("  ! договор ДП-77/2026: %v", err)
	}
	log.Println("► договоры созданы")

	// --- releases (partial draws so progress is visible) ---
	if c1 != nil {
		if _, err := svc.Release.Create(ctx, releasesvc.CreateInput{
			ContractID: c1.ID, Date: now.AddDate(0, 0, -3), Note: "Первая партия по договору",
			CreatedBy: admin.ID,
			Lines: []releasesvc.LineInput{
				{ContractLineID: c1.Lines[0].ID, PositionID: pos["neocate"], Quantity: 20},
				{ContractLineID: c1.Lines[1].ID, PositionID: pos["pediasure"], Quantity: 60},
			},
		}); err != nil {
			log.Printf("  ! отпуск по ГЗ-2026-014: %v", err)
		}
	}
	if c2 != nil {
		if _, err := svc.Release.Create(ctx, releasesvc.CreateInput{
			ContractID: c2.ID, Date: now, Note: "Отпуск по заявке",
			CreatedBy: admin.ID,
			Lines: []releasesvc.LineInput{
				{ContractLineID: c2.Lines[0].ID, PositionID: pos["alfamino"], Quantity: 10},
			},
		}); err != nil {
			log.Printf("  ! отпуск по ДП-77/2026: %v", err)
		}
	}
	log.Println("► отпуск по договорам проведён")

	printCreds(admin.Email, password)
	return nil
}

func printCreds(email, password string) {
	fmt.Println()
	fmt.Println("──────────────────────────────────────────────")
	fmt.Println(" Готово. Вход на фронтенде (http://localhost:3000):")
	fmt.Printf("   супер-админ  : %s / %s\n", email, password)
	fmt.Println("   администратор: manager@saubala.kz / manager123")
	fmt.Println("   сотрудник    : sklad@saubala.kz / sklad12345")
	fmt.Println("──────────────────────────────────────────────")
}
