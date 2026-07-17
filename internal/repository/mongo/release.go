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
	ContractLineID string `bson:"contract_line_id,omitempty"`
	PositionID     string `bson:"position_id"`
	Quantity       int    `bson:"quantity"`
	UnitCost       int64  `bson:"unit_cost"`
	UnitPrice      int64  `bson:"unit_price,omitempty"`
}

type releaseDoc struct {
	ID               string           `bson:"_id"`
	ContractID       string           `bson:"contract_id,omitempty"`
	Date             time.Time        `bson:"date"`
	Note             string           `bson:"note"`
	DocumentNumber   string           `bson:"document_number,omitempty"`
	RecipientName    string           `bson:"recipient_name,omitempty"`
	RecipientAddress string           `bson:"recipient_address,omitempty"`
	OrganizationID   string           `bson:"organization_id,omitempty"`
	Lines            []releaseLineDoc `bson:"lines"`
	CreatedBy        string           `bson:"created_by"`
	CreatedAt        time.Time        `bson:"created_at"`
}

func toReleaseDoc(r *release.Release) releaseDoc {
	lines := make([]releaseLineDoc, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = releaseLineDoc{
			ContractLineID: l.ContractLineID,
			PositionID:     l.PositionID,
			Quantity:       l.Quantity,
			UnitCost:       l.UnitCost,
			UnitPrice:      l.UnitPrice,
		}
	}
	return releaseDoc{
		ID:               r.ID,
		ContractID:       r.ContractID,
		Date:             r.Date,
		Note:             r.Note,
		DocumentNumber:   r.DocumentNumber,
		RecipientName:    r.RecipientName,
		RecipientAddress: r.RecipientAddress,
		OrganizationID:   r.OrganizationID,
		Lines:            lines,
		CreatedBy:        r.CreatedBy,
		CreatedAt:        r.CreatedAt,
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
			UnitPrice:      l.UnitPrice,
		}
	}
	return &release.Release{
		ID:               d.ID,
		ContractID:       d.ContractID,
		Date:             d.Date,
		Note:             d.Note,
		DocumentNumber:   d.DocumentNumber,
		RecipientName:    d.RecipientName,
		RecipientAddress: d.RecipientAddress,
		OrganizationID:   d.OrganizationID,
		Lines:            lines,
		CreatedBy:        d.CreatedBy,
		CreatedAt:        d.CreatedAt,
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

func (r *ReleaseRepository) UpdateWaybill(ctx context.Context, id string, u release.WaybillUpdate) error {
	set := bson.M{}
	if u.DocumentNumber != nil {
		set["document_number"] = *u.DocumentNumber
	}
	if u.RecipientName != nil {
		set["recipient_name"] = *u.RecipientName
	}
	if u.RecipientAddress != nil {
		set["recipient_address"] = *u.RecipientAddress
	}
	if u.OrganizationID != nil {
		set["organization_id"] = *u.OrganizationID
	}
	if len(set) == 0 {
		return nil
	}
	res, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *ReleaseRepository) List(ctx context.Context, f release.Filter) ([]release.Release, int64, error) {
	filter := bson.M{}
	if f.ContractID != "" {
		filter["contract_id"] = f.ContractID
	} else if f.OnlyFree {
		filter["$or"] = bson.A{
			bson.M{"contract_id": bson.M{"$exists": false}},
			bson.M{"contract_id": ""},
		}
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

func (r *ReleaseRepository) ReleasedByContracts(ctx context.Context, contractIDs []string) (map[string]map[string]int, error) {
	if len(contractIDs) == 0 {
		return map[string]map[string]int{}, nil
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"contract_id": bson.M{"$in": contractIDs}}}},
		bson.D{{Key: "$unwind", Value: "$lines"}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"contract_id": "$contract_id",
				"line_id":     "$lines.contract_line_id",
			},
			"total": bson.M{"$sum": "$lines.quantity"},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		ID struct {
			ContractID string `bson:"contract_id"`
			LineID     string `bson:"line_id"`
		} `bson:"_id"`
		Total int `bson:"total"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}

	out := make(map[string]map[string]int)
	for _, row := range rows {
		m := out[row.ID.ContractID]
		if m == nil {
			m = make(map[string]int)
			out[row.ID.ContractID] = m
		}
		m[row.ID.LineID] = row.Total
	}
	return out, nil
}

func (r *ReleaseRepository) CountByOrganization(ctx context.Context, organizationID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"organization_id": organizationID})
}
