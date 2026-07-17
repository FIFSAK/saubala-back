package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/FIFSAK/saubala-back/internal/domain/adjustment"
)

type adjustmentDoc struct {
	ID         string    `bson:"_id"`
	PositionID string    `bson:"position_id"`
	Delta      int       `bson:"delta"`
	Note       string    `bson:"note"`
	CreatedBy  string    `bson:"created_by"`
	CreatedAt  time.Time `bson:"created_at"`
}

func toAdjustmentDoc(a *adjustment.Adjustment) adjustmentDoc {
	return adjustmentDoc{
		ID:         a.ID,
		PositionID: a.PositionID,
		Delta:      a.Delta,
		Note:       a.Note,
		CreatedBy:  a.CreatedBy,
		CreatedAt:  a.CreatedAt,
	}
}

func (d adjustmentDoc) toDomain() *adjustment.Adjustment {
	return &adjustment.Adjustment{
		ID:         d.ID,
		PositionID: d.PositionID,
		Delta:      d.Delta,
		Note:       d.Note,
		CreatedBy:  d.CreatedBy,
		CreatedAt:  d.CreatedAt,
	}
}

// AdjustmentRepository is the MongoDB implementation of adjustment.Repository.
type AdjustmentRepository struct {
	coll *mongo.Collection
}

func NewAdjustmentRepository(db *mongo.Database) *AdjustmentRepository {
	return &AdjustmentRepository{coll: db.Collection(collAdjustments)}
}

func (r *AdjustmentRepository) Create(ctx context.Context, a *adjustment.Adjustment) error {
	_, err := r.coll.InsertOne(ctx, toAdjustmentDoc(a))
	return err
}

func (r *AdjustmentRepository) ListByPosition(ctx context.Context, positionID string) ([]adjustment.Adjustment, error) {
	cur, err := r.coll.Find(ctx, bson.M{"position_id": positionID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []adjustmentDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]adjustment.Adjustment, len(docs))
	for i, d := range docs {
		out[i] = *d.toDomain()
	}
	return out, nil
}
