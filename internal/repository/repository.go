package repository

import (
	"github.com/FIFSAK/saubala-back/internal/domain/shipment"
	sqliteRepo "github.com/FIFSAK/saubala-back/internal/repository/sqlite"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type Configuration func(r *Repositories) error

type Repositories struct {
	store *store.SQL

	Shipment shipment.Repository
}

func New(configs ...Configuration) (s *Repositories, err error) {
	s = &Repositories{}

	for _, cfg := range configs {
		if err = cfg(s); err != nil {
			return
		}
	}

	return
}

func (r *Repositories) Close() {
	if r.store != nil && r.store.Connection != nil {
		r.store.Connection.Close()
	}
}

func WithSQLiteStore(dataSourceName string) Configuration {
	return func(s *Repositories) (err error) {
		s.store, err = store.NewSQL(dataSourceName)
		if err != nil {
			return
		}

		if err = store.RunMigrations(dataSourceName); err != nil {
			return
		}

		s.Shipment = sqliteRepo.NewShipmentRepository(s.store.Connection)

		return
	}
}
