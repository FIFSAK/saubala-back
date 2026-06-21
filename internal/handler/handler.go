package handler

import (
	"github.com/go-chi/chi/v5"

	"github.com/FIFSAK/saubala-back/internal/config"
	restHandler "github.com/FIFSAK/saubala-back/internal/handler/rest"
	"github.com/FIFSAK/saubala-back/internal/service"
)

type Dependencies struct {
	Configs  *config.Configs
	Services *service.Services
}

type Configuration func(h *Handlers) error

type Handlers struct {
	dependencies Dependencies
	Shipment     *restHandler.ShipmentHandler
}

func New(d Dependencies, configs ...Configuration) (h *Handlers, err error) {
	h = &Handlers{
		dependencies: d,
	}

	for _, cfg := range configs {
		if err = cfg(h); err != nil {
			return
		}
	}

	return
}

func WithShipmentHandler() Configuration {
	return func(h *Handlers) error {
		h.Shipment = restHandler.NewShipmentHandler(h.dependencies.Services.Shipment)
		return nil
	}
}

// RegisterHTTP mounts all handlers under the versioned /api/v1 group.
func (h *Handlers) RegisterHTTP(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		if h.Shipment != nil {
			h.Shipment.Register(r)
		}
	})
}
