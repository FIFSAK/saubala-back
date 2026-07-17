package repository

import (
	"context"

	"github.com/FIFSAK/saubala-back/internal/domain/adjustment"
	"github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/internal/domain/settings"
	"github.com/FIFSAK/saubala-back/internal/domain/user"
	mongorepo "github.com/FIFSAK/saubala-back/internal/repository/mongo"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

// Configuration mutates the Repositories aggregate during construction.
type Configuration func(r *Repositories) error

// Repositories is the aggregate of all persistence ports used by the services.
type Repositories struct {
	mongo *store.Mongo

	User       user.Repository
	Brand      brand.Repository
	Position   position.Repository
	Receipt    receipt.Repository
	Contract   contract.Repository
	Release    release.Repository
	Adjustment adjustment.Repository
	Settings   settings.Repository
}

// New builds the repositories aggregate from the given options.
func New(configs ...Configuration) (*Repositories, error) {
	r := &Repositories{}
	for _, cfg := range configs {
		if err := cfg(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Close releases the underlying store resources.
func (r *Repositories) Close(ctx context.Context) error {
	if r.mongo != nil {
		return r.mongo.Close(ctx)
	}
	return nil
}

// WithMongoStore wires every repository onto the given Mongo store and ensures
// the required indexes exist (replacing schema migrations).
func WithMongoStore(ctx context.Context, m *store.Mongo) Configuration {
	return func(r *Repositories) error {
		r.mongo = m
		db := m.DB
		if err := mongorepo.EnsureIndexes(ctx, db); err != nil {
			return err
		}
		r.User = mongorepo.NewUserRepository(db)
		r.Brand = mongorepo.NewBrandRepository(db)
		r.Position = mongorepo.NewPositionRepository(db)
		r.Receipt = mongorepo.NewReceiptRepository(db)
		r.Contract = mongorepo.NewContractRepository(db)
		r.Release = mongorepo.NewReleaseRepository(db)
		r.Adjustment = mongorepo.NewAdjustmentRepository(db)
		r.Settings = mongorepo.NewSettingsRepository(db)
		return nil
	}
}
