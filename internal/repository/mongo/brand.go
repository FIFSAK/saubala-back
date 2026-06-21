package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/brand"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type brandDoc struct {
	ID        string     `bson:"_id"`
	Name      string     `bson:"name"`
	DeletedAt *time.Time `bson:"deleted_at"`
	CreatedAt time.Time  `bson:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at"`
}

func toBrandDoc(b *brand.Brand) brandDoc {
	return brandDoc{
		ID:        b.ID,
		Name:      b.Name,
		DeletedAt: b.DeletedAt,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

func (d brandDoc) toDomain() *brand.Brand {
	return &brand.Brand{
		ID:        d.ID,
		Name:      d.Name,
		DeletedAt: d.DeletedAt,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// BrandRepository is the MongoDB implementation of brand.Repository.
type BrandRepository struct {
	coll *mongo.Collection
}

func NewBrandRepository(db *mongo.Database) *BrandRepository {
	return &BrandRepository{coll: db.Collection(collBrands)}
}

var brandSortFields = map[string]string{
	"name":       "name",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// notDeleted constrains a query to brands that have not been soft-deleted.
func notDeleted(extra bson.M) bson.M {
	filter := bson.M{"deleted_at": bson.M{"$eq": nil}}
	for k, v := range extra {
		filter[k] = v
	}
	return filter
}

func (r *BrandRepository) Create(ctx context.Context, b *brand.Brand) error {
	_, err := r.coll.InsertOne(ctx, toBrandDoc(b))
	if store.IsDuplicateKey(err) {
		return store.ErrDuplicate
	}
	return err
}

func (r *BrandRepository) GetByID(ctx context.Context, id string) (*brand.Brand, error) {
	return r.findOne(ctx, notDeleted(bson.M{"_id": id}))
}

func (r *BrandRepository) GetByName(ctx context.Context, name string) (*brand.Brand, error) {
	return r.findOne(ctx, notDeleted(bson.M{"name": name}))
}

func (r *BrandRepository) findOne(ctx context.Context, filter bson.M) (*brand.Brand, error) {
	var d brandDoc
	if err := r.coll.FindOne(ctx, filter).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *BrandRepository) Update(ctx context.Context, b *brand.Brand) error {
	b.UpdatedAt = time.Now().UTC()
	res, err := r.coll.UpdateOne(ctx,
		notDeleted(bson.M{"_id": b.ID}),
		bson.M{"$set": bson.M{"name": b.Name, "updated_at": b.UpdatedAt}},
	)
	if store.IsDuplicateKey(err) {
		return store.ErrDuplicate
	}
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *BrandRepository) SoftDelete(ctx context.Context, id string) error {
	now := time.Now().UTC()
	res, err := r.coll.UpdateOne(ctx,
		notDeleted(bson.M{"_id": id}),
		bson.M{"$set": bson.M{"deleted_at": now, "updated_at": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *BrandRepository) List(ctx context.Context, f brand.Filter) ([]brand.Brand, int64, error) {
	filter := notDeleted(nil)
	if f.Q != "" {
		filter["name"] = caseInsensitiveContains(f.Q)
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, brandSortFields, "created_at"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []brandDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	brands := make([]brand.Brand, len(docs))
	for i, d := range docs {
		brands[i] = *d.toDomain()
	}
	return brands, total, nil
}
