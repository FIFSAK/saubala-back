package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/FIFSAK/saubala-back/internal/handler/rest"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	"github.com/FIFSAK/saubala-back/internal/repository"
	"github.com/FIFSAK/saubala-back/internal/service"
	"github.com/FIFSAK/saubala-back/pkg/auth"
)

// Dependencies are the inputs required to build the HTTP handlers.
type Dependencies struct {
	Services     *service.Services
	Repositories *repository.Repositories
	TokenManager *auth.TokenManager
}

// Configuration mutates the Handlers aggregate during construction.
type Configuration func(h *Handlers) error

// Handlers is the aggregate of all REST handlers.
type Handlers struct {
	deps Dependencies

	Auth     *rest.AuthHandler
	User     *rest.UserHandler
	Brand    *rest.BrandHandler
	Position *rest.PositionHandler
	Receipt  *rest.ReceiptHandler
	Contract *rest.ContractHandler
	Release  *rest.ReleaseHandler
	Settings *rest.SettingsHandler
	Org      *rest.OrgHandler
	Supplier *rest.SupplierHandler
	Invoice  *rest.InvoiceHandler
}

// New builds the handlers aggregate from the given options.
func New(d Dependencies, configs ...Configuration) (*Handlers, error) {
	h := &Handlers{deps: d}
	for _, cfg := range configs {
		if err := cfg(h); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func WithAuthHandler() Configuration {
	return func(h *Handlers) error {
		h.Auth = rest.NewAuthHandler(h.deps.Services.Auth)
		return nil
	}
}

func WithUserHandler() Configuration {
	return func(h *Handlers) error {
		h.User = rest.NewUserHandler(h.deps.Services.User)
		return nil
	}
}

func WithBrandHandler() Configuration {
	return func(h *Handlers) error {
		h.Brand = rest.NewBrandHandler(h.deps.Services.Brand)
		return nil
	}
}

func WithPositionHandler() Configuration {
	return func(h *Handlers) error {
		h.Position = rest.NewPositionHandler(h.deps.Services.Position)
		return nil
	}
}

func WithReceiptHandler() Configuration {
	return func(h *Handlers) error {
		h.Receipt = rest.NewReceiptHandler(h.deps.Services.Receipt)
		return nil
	}
}

func WithContractHandler() Configuration {
	return func(h *Handlers) error {
		h.Contract = rest.NewContractHandler(h.deps.Services.Contract)
		return nil
	}
}

func WithReleaseHandler() Configuration {
	return func(h *Handlers) error {
		h.Release = rest.NewReleaseHandler(h.deps.Services.Release)
		return nil
	}
}

func WithSettingsHandler() Configuration {
	return func(h *Handlers) error {
		h.Settings = rest.NewSettingsHandler(h.deps.Services.Settings)
		return nil
	}
}

func WithOrgHandler() Configuration {
	return func(h *Handlers) error {
		h.Org = rest.NewOrgHandler(h.deps.Services.Org)
		return nil
	}
}

func WithSupplierHandler() Configuration {
	return func(h *Handlers) error {
		h.Supplier = rest.NewSupplierHandler(h.deps.Services.Supplier)
		return nil
	}
}

func WithInvoiceHandler() Configuration {
	return func(h *Handlers) error {
		h.Invoice = rest.NewInvoiceHandler(h.deps.Services.Invoice)
		return nil
	}
}

// RegisterHTTP mounts all routes under /api/v1, applying authentication to every
// route except login, and the admin guard to user-management routes.
func (h *Handlers) RegisterHTTP(r chi.Router) {
	authenticate := middleware.Authenticator(h.deps.TokenManager, h.deps.Repositories.User)

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes.
		if h.Auth != nil {
			r.Post("/auth/login", h.Auth.Login)
		}

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(authenticate)

			if h.Auth != nil {
				r.Get("/auth/me", h.Auth.Me)
			}

			// User management, settings and organization writes are restricted
			// to admins.
			if h.User != nil || h.Settings != nil || h.Org != nil {
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireAdmin)
					if h.User != nil {
						h.User.Register(r)
					}
					if h.Settings != nil {
						h.Settings.RegisterWrite(r)
					}
					if h.Org != nil {
						h.Org.RegisterWrite(r)
					}
				})
			}

			if h.Settings != nil {
				h.Settings.RegisterRead(r)
			}
			if h.Org != nil {
				h.Org.RegisterRead(r)
			}
			if h.Invoice != nil {
				h.Invoice.Register(r)
			}

			if h.Brand != nil {
				h.Brand.Register(r)
			}
			if h.Supplier != nil {
				h.Supplier.Register(r)
			}
			if h.Position != nil {
				h.Position.Register(r)
			}
			if h.Receipt != nil {
				h.Receipt.Register(r)
			}
			if h.Contract != nil {
				h.Contract.Register(r)
			}
			if h.Release != nil {
				h.Release.Register(r)
			}
		})
	})
}
