package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type releaseLineDoc struct {
	ContractLineID string `bson:"contract_line_id"`
	PositionID     string `bson:"position_id"`
	Quantity       int    `bson:"quantity"`
	UnitCost       int64  `bson:"unit_cost"`
}

type releaseDoc struct {
	ID         string           `bson:"_id"`
	ContractID string           `bson:"contract_id"`
	Date       time.Time        `bson:"date"`
	Note       string           `bson:"note"`
	Lines      []releaseLineDoc `bson:"lines"`
	CreatedBy  string           `bson:"created_by"`
	CreatedAt  time.Time        `bson:"created_at"`
}

func toReleaseDoc(r *release.Release) releaseDoc {
	lines := make([]releaseLineDoc, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = releaseLineDoc{
			ContractLineID: l.ContractLineID,
			PositionID:     l.PositionID,
			Quantity:       l.Quantity,
			UnitCost:       l.UnitCost,
		}
	}
	return releaseDoc{
		ID:         r.ID,
		ContractID: r.ContractID,
		Date:       r.Date,
		Note:       r.Note,
		Lines:      lines,
		CreatedBy:  r.CreatedBy,
		CreatedAt:  r.CreatedAt,
	}
}

func (d releaseDoc) toDomain() *release.Release {
	lines := make([]release.Line, len(d.Lines))
	for i, l := range d.Lines {
		lines[i] = release.Line{
			ContractLineID: l.ContractLineID,
			PositionID:     l.PositionID,
			Quantity:       l.Quantity,
			UnitCost:       l.UnitCost,
		}
	}
	return &release.Release{
		ID:         d.ID,
		ContractID: d.ContractID,
		Date:       d.Date,
		Note:       d.Note,
		Lines:      lines,
		CreatedBy:  d.CreatedBy,
		CreatedAt:  d.CreatedAt,
	}
}

// ReleaseRepository is the MongoDB implementation of release.Repository.
type ReleaseRepository struct {
	coll *mongo.Collection
}

func NewReleaseRepository(db *mongo.Database) *ReleaseRepository {
	return &ReleaseRepository{coll: db.Collection(collReleases)}
}

var releaseSortFields = map[string]string{
	"date":       "date",
	"created_at": "created_at",
}

func (r *ReleaseRepository) Create(ctx context.Context, rel *release.Release) error {
	_, err := r.coll.InsertOne(ctx, toReleaseDoc(rel))
	return err
}

func (r *ReleaseRepository) GetByID(ctx context.Context, id string) (*release.Release, error) {
	var d releaseDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *ReleaseRepository) List(ctx context.Context, f release.Filter) ([]release.Release, int64, error) {
	filter := bson.M{}
	if f.ContractID != "" {
		filter["contract_id"] = f.ContractID
	}
	if f.DateFrom != nil || f.DateTo != nil {
		date := bson.M{}
		if f.DateFrom != nil {
			date["$gte"] = *f.DateFrom
		}
		if f.DateTo != nil {
			date["$lte"] = *f.DateTo
		}
		filter["date"] = date
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, releaseSortFields, "date"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []releaseDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	releases := make([]release.Release, len(docs))
	for i, d := range docs {
		releases[i] = *d.toDomain()
	}
	return releases, total, nil
}

func (r *ReleaseRepository) ListByPosition(ctx context.Context, positionID string) ([]release.Release, error) {
	cur, err := r.coll.Find(ctx, bson.M{"lines.position_id": positionID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []releaseDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	releases := make([]release.Release, len(docs))
	for i, d := range docs {
		releases[i] = *d.toDomain()
	}
	return releases, nil
}

func (r *ReleaseRepository) CountByContract(ctx context.Context, contractID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"contract_id": contractID})
}

func (r *ReleaseRepository) ReleasedByContract(ctx context.Context, contractID string) (map[string]int, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"contract_id": contractID}}},
		bson.D{{Key: "$unwind", Value: "$lines"}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":   "$lines.contract_line_id",
			"total": bson.M{"$sum": "$lines.quantity"},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		ID    string `bson:"_id"`
		Total int    `bson:"total"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}

	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Total
	}
	return out, nil
}
