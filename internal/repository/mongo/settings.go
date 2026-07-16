package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/settings"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type settingsDoc struct {
	ID                    string    `bson:"_id"`
	OrgName               string    `bson:"org_name"`
	BIN                   string    `bson:"bin"`
	ResponsibleForSupply  string    `bson:"responsible_for_supply"`
	Director              string    `bson:"director"`
	Accountant            string    `bson:"accountant"`
	VATRatePercent        int       `bson:"vat_rate_percent"`
	LineDescriptionPrefix string    `bson:"line_description_prefix"`
	DefaultUnit           string    `bson:"default_unit"`
	UpdatedAt             time.Time `bson:"updated_at"`
}

func toSettingsDoc(o *settings.Organization) settingsDoc {
	return settingsDoc{
		ID:                    settings.ID,
		OrgName:               o.OrgName,
		BIN:                   o.BIN,
		ResponsibleForSupply:  o.ResponsibleForSupply,
		Director:              o.Director,
		Accountant:            o.Accountant,
		VATRatePercent:        o.VATRatePercent,
		LineDescriptionPrefix: o.LineDescriptionPrefix,
		DefaultUnit:           o.DefaultUnit,
		UpdatedAt:             o.UpdatedAt,
	}
}

func (d settingsDoc) toDomain() *settings.Organization {
	return &settings.Organization{
		OrgName:               d.OrgName,
		BIN:                   d.BIN,
		ResponsibleForSupply:  d.ResponsibleForSupply,
		Director:              d.Director,
		Accountant:            d.Accountant,
		VATRatePercent:        d.VATRatePercent,
		LineDescriptionPrefix: d.LineDescriptionPrefix,
		DefaultUnit:           d.DefaultUnit,
		UpdatedAt:             d.UpdatedAt,
	}
}

// SettingsRepository is the MongoDB implementation of settings.Repository.
type SettingsRepository struct {
	coll *mongo.Collection
}

func NewSettingsRepository(db *mongo.Database) *SettingsRepository {
	return &SettingsRepository{coll: db.Collection(collSettings)}
}

func (r *SettingsRepository) Get(ctx context.Context) (*settings.Organization, error) {
	var d settingsDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": settings.ID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *SettingsRepository) Upsert(ctx context.Context, o *settings.Organization) error {
	doc := toSettingsDoc(o)
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": settings.ID},
		bson.M{"$set": doc},
		options.Update().SetUpsert(true),
	)
	return err
}
