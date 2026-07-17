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

// settingsDoc keeps only the invoice defaults; legacy seller fields that older
// versions stored on the same document are simply ignored on decode.
type settingsDoc struct {
	ID                    string    `bson:"_id"`
	VATRatePercent        int       `bson:"vat_rate_percent"`
	LineDescriptionPrefix string    `bson:"line_description_prefix"`
	DefaultUnit           string    `bson:"default_unit"`
	UpdatedAt             time.Time `bson:"updated_at"`
}

func toSettingsDoc(s *settings.Settings) settingsDoc {
	return settingsDoc{
		ID:                    settings.ID,
		VATRatePercent:        s.VATRatePercent,
		LineDescriptionPrefix: s.LineDescriptionPrefix,
		DefaultUnit:           s.DefaultUnit,
		UpdatedAt:             s.UpdatedAt,
	}
}

func (d settingsDoc) toDomain() *settings.Settings {
	return &settings.Settings{
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

func (r *SettingsRepository) Get(ctx context.Context) (*settings.Settings, error) {
	var d settingsDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": settings.ID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *SettingsRepository) Upsert(ctx context.Context, s *settings.Settings) error {
	doc := toSettingsDoc(s)
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": settings.ID},
		bson.M{"$set": doc},
		options.Update().SetUpsert(true),
	)
	return err
}
