package service

import (
	"github.com/FIFSAK/saubala-back/internal/repository"
	authsvc "github.com/FIFSAK/saubala-back/internal/service/auth"
	brandsvc "github.com/FIFSAK/saubala-back/internal/service/brand"
	contractsvc "github.com/FIFSAK/saubala-back/internal/service/contract"
	invoicesvc "github.com/FIFSAK/saubala-back/internal/service/invoice"
	positionsvc "github.com/FIFSAK/saubala-back/internal/service/position"
	receiptsvc "github.com/FIFSAK/saubala-back/internal/service/receipt"
	releasesvc "github.com/FIFSAK/saubala-back/internal/service/release"
	settingssvc "github.com/FIFSAK/saubala-back/internal/service/settings"
	usersvc "github.com/FIFSAK/saubala-back/internal/service/user"
	"github.com/FIFSAK/saubala-back/pkg/auth"
)

// Dependencies are the inputs required to build the service aggregate.
type Dependencies struct {
	Repositories *repository.Repositories
	TokenManager *auth.TokenManager
}

// Configuration mutates the Services aggregate during construction.
type Configuration func(s *Services) error

// Services is the aggregate of all application services.
type Services struct {
	deps Dependencies

	Auth     *authsvc.Service
	User     *usersvc.Service
	Brand    *brandsvc.Service
	Position *positionsvc.Service
	Receipt  *receiptsvc.Service
	Contract *contractsvc.Service
	Release  *releasesvc.Service
	Settings *settingssvc.Service
	Invoice  *invoicesvc.Service
}

// New builds the services aggregate from the given options.
func New(deps Dependencies, configs ...Configuration) (*Services, error) {
	s := &Services{deps: deps}
	for _, cfg := range configs {
		if err := cfg(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func WithAuthService() Configuration {
	return func(s *Services) error {
		s.Auth = authsvc.NewService(s.deps.Repositories.User, s.deps.TokenManager)
		return nil
	}
}

func WithUserService() Configuration {
	return func(s *Services) error {
		s.User = usersvc.NewService(s.deps.Repositories.User)
		return nil
	}
}

func WithBrandService() Configuration {
	return func(s *Services) error {
		s.Brand = brandsvc.NewService(s.deps.Repositories.Brand, s.deps.Repositories.Position)
		return nil
	}
}

func WithPositionService() Configuration {
	return func(s *Services) error {
		s.Position = positionsvc.NewService(
			s.deps.Repositories.Position,
			s.deps.Repositories.Brand,
			s.deps.Repositories.Receipt,
			s.deps.Repositories.Release,
			s.deps.Repositories.Contract,
			s.deps.Repositories.Adjustment,
		)
		return nil
	}
}

func WithReceiptService() Configuration {
	return func(s *Services) error {
		s.Receipt = receiptsvc.NewService(s.deps.Repositories.Receipt, s.deps.Repositories.Position)
		return nil
	}
}

func WithContractService() Configuration {
	return func(s *Services) error {
		s.Contract = contractsvc.NewService(
			s.deps.Repositories.Contract,
			s.deps.Repositories.Position,
			s.deps.Repositories.Release,
		)
		return nil
	}
}

func WithReleaseService() Configuration {
	return func(s *Services) error {
		s.Release = releasesvc.NewService(
			s.deps.Repositories.Release,
			s.deps.Repositories.Contract,
			s.deps.Repositories.Position,
		)
		return nil
	}
}

func WithSettingsService() Configuration {
	return func(s *Services) error {
		s.Settings = settingssvc.NewService(s.deps.Repositories.Settings)
		return nil
	}
}

func WithInvoiceService() Configuration {
	return func(s *Services) error {
		s.Invoice = invoicesvc.NewService(
			s.deps.Repositories.Release,
			s.deps.Repositories.Contract,
			s.deps.Repositories.Position,
			s.deps.Repositories.Settings,
		)
		return nil
	}
}
