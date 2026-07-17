package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/supplier"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type supplierDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Type      string    `bson:"type"`
	BIN       string    `bson:"bin,omitempty"`
	Country   string    `bson:"country,omitempty"`
	Phone     string    `bson:"phone,omitempty"`
	Email     string    `bson:"email,omitempty"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func toSupplierDoc(s *supplier.Supplier) supplierDoc {
	return supplierDoc{
		ID:        s.ID,
		Name:      s.Name,
		Type:      string(s.Type),
		BIN:       s.BIN,
		Country:   s.Country,
		Phone:     s.Phone,
		Email:     s.Email,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func (d supplierDoc) toDomain() *supplier.Supplier {
	return &supplier.Supplier{
		ID:        d.ID,
		Name:      d.Name,
		Type:      supplier.Type(d.Type),
		BIN:       d.BIN,
		Country:   d.Country,
		Phone:     d.Phone,
		Email:     d.Email,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// SupplierRepository is the MongoDB implementation of supplier.Repository.
type SupplierRepository struct {
	coll *mongo.Collection
}

func NewSupplierRepository(db *mongo.Database) *SupplierRepository {
	return &SupplierRepository{coll: db.Collection(collSuppliers)}
}

var supplierSortFields = map[string]string{
	"name":       "name",
	"created_at": "created_at",
}

func (r *SupplierRepository) Create(ctx context.Context, s *supplier.Supplier) error {
	_, err := r.coll.InsertOne(ctx, toSupplierDoc(s))
	return err
}

func (r *SupplierRepository) GetByID(ctx context.Context, id string) (*supplier.Supplier, error) {
	var d supplierDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *SupplierRepository) GetByIDs(ctx context.Context, ids []string) ([]supplier.Supplier, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []supplierDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]supplier.Supplier, len(docs))
	for i, d := range docs {
		out[i] = *d.toDomain()
	}
	return out, nil
}

func (r *SupplierRepository) Update(ctx context.Context, s *supplier.Supplier) error {
	s.UpdatedAt = time.Now().UTC()
	doc := toSupplierDoc(s)
	res, err := r.coll.UpdateByID(ctx, s.ID, bson.M{"$set": bson.M{
		"name":       doc.Name,
		"type":       doc.Type,
		"bin":        doc.BIN,
		"country":    doc.Country,
		"phone":      doc.Phone,
		"email":      doc.Email,
		"updated_at": doc.UpdatedAt,
	}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *SupplierRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *SupplierRepository) List(ctx context.Context, f supplier.Filter) ([]supplier.Supplier, int64, error) {
	filter := bson.M{}
	if f.Q != "" {
		rx := caseInsensitiveContains(f.Q)
		filter["$or"] = bson.A{
			bson.M{"name": rx},
			bson.M{"bin": rx},
			bson.M{"country": rx},
		}
	}
	if f.Type != "" {
		filter["type"] = string(f.Type)
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, supplierSortFields, "name"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []supplierDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	suppliers := make([]supplier.Supplier, len(docs))
	for i, d := range docs {
		suppliers[i] = *d.toDomain()
	}
	return suppliers, total, nil
}
