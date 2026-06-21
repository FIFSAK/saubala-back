package service

import (
	"github.com/FIFSAK/saubala-back/internal/config"
	"github.com/FIFSAK/saubala-back/internal/repository"
	shipmentSvc "github.com/FIFSAK/saubala-back/internal/service/shipment"
)

type Dependencies struct {
	Repositories *repository.Repositories
	Configs      *config.Configs
}

type Configuration func(s *Services) error

type Services struct {
	dependencies Dependencies
	Shipment     shipmentSvc.Service
}

func New(dependencies Dependencies, configs ...Configuration) (s *Services, err error) {
	s = &Services{
		dependencies: dependencies,
	}

	for _, cfg := range configs {
		if err = cfg(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func WithShipmentService() Configuration {
	return func(s *Services) error {
		s.Shipment = shipmentSvc.NewShipmentService(
			s.dependencies.Repositories.Shipment,
		)
		return nil
	}
}
