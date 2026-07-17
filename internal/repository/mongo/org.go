package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/org"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type orgDoc struct {
	ID                   string    `bson:"_id"`
	Name                 string    `bson:"name"`
	BIN                  string    `bson:"bin"`
	ResponsibleForSupply string    `bson:"responsible_for_supply"`
	Director             string    `bson:"director"`
	Accountant           string    `bson:"accountant"`
	CreatedAt            time.Time `bson:"created_at"`
	UpdatedAt            time.Time `bson:"updated_at"`
}

func toOrgDoc(o *org.Organization) orgDoc {
	return orgDoc{
		ID:                   o.ID,
		Name:                 o.Name,
		BIN:                  o.BIN,
		ResponsibleForSupply: o.ResponsibleForSupply,
		Director:             o.Director,
		Accountant:           o.Accountant,
		CreatedAt:            o.CreatedAt,
		UpdatedAt:            o.UpdatedAt,
	}
}

func (d orgDoc) toDomain() *org.Organization {
	return &org.Organization{
		ID:                   d.ID,
		Name:                 d.Name,
		BIN:                  d.BIN,
		ResponsibleForSupply: d.ResponsibleForSupply,
		Director:             d.Director,
		Accountant:           d.Accountant,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
	}
}

// OrgRepository is the MongoDB implementation of org.Repository.
type OrgRepository struct {
	coll *mongo.Collection
}

func NewOrgRepository(db *mongo.Database) *OrgRepository {
	return &OrgRepository{coll: db.Collection(collOrganizations)}
}

func (r *OrgRepository) Create(ctx context.Context, o *org.Organization) error {
	_, err := r.coll.InsertOne(ctx, toOrgDoc(o))
	return err
}

func (r *OrgRepository) GetByID(ctx context.Context, id string) (*org.Organization, error) {
	var d orgDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *OrgRepository) GetByIDs(ctx context.Context, ids []string) ([]org.Organization, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []orgDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]org.Organization, len(docs))
	for i, d := range docs {
		out[i] = *d.toDomain()
	}
	return out, nil
}

func (r *OrgRepository) Update(ctx context.Context, o *org.Organization) error {
	o.UpdatedAt = time.Now().UTC()
	doc := toOrgDoc(o)
	res, err := r.coll.UpdateByID(ctx, o.ID, bson.M{"$set": bson.M{
		"name":                   doc.Name,
		"bin":                    doc.BIN,
		"responsible_for_supply": doc.ResponsibleForSupply,
		"director":               doc.Director,
		"accountant":             doc.Accountant,
		"updated_at":             doc.UpdatedAt,
	}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *OrgRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *OrgRepository) List(ctx context.Context) ([]org.Organization, error) {
	cur, err := r.coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []orgDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]org.Organization, len(docs))
	for i, d := range docs {
		out[i] = *d.toDomain()
	}
	return out, nil
}

func (r *OrgRepository) Count(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{})
}
