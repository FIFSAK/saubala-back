package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/position"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type positionDoc struct {
	ID            string    `bson:"_id"`
	Name          string    `bson:"name"`
	BrandID       string    `bson:"brand_id"`
	ContractName  string    `bson:"contract_name"`
	ExpiryDate    time.Time `bson:"expiry_date"`
	LotNumber     string    `bson:"lot_number"`
	PurchasePrice int64     `bson:"purchase_price"`
	Quantity      int       `bson:"quantity"`
	MassGrams     int       `bson:"mass_grams"`
	CreatedAt     time.Time `bson:"created_at"`
	UpdatedAt     time.Time `bson:"updated_at"`
}

func toPositionDoc(p *position.Position) positionDoc {
	return positionDoc{
		ID:            p.ID,
		Name:          p.Name,
		BrandID:       p.BrandID,
		ContractName:  p.ContractName,
		ExpiryDate:    p.ExpiryDate,
		LotNumber:     p.LotNumber,
		PurchasePrice: p.PurchasePrice,
		Quantity:      p.Quantity,
		MassGrams:     p.MassGrams,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

func (d positionDoc) toDomain() *position.Position {
	return &position.Position{
		ID:            d.ID,
		Name:          d.Name,
		BrandID:       d.BrandID,
		ContractName:  d.ContractName,
		ExpiryDate:    d.ExpiryDate,
		LotNumber:     d.LotNumber,
		PurchasePrice: d.PurchasePrice,
		Quantity:      d.Quantity,
		MassGrams:     d.MassGrams,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// PositionRepository is the MongoDB implementation of position.Repository.
type PositionRepository struct {
	coll *mongo.Collection
}

func NewPositionRepository(db *mongo.Database) *PositionRepository {
	return &PositionRepository{coll: db.Collection(collPositions)}
}

var positionSortFields = map[string]string{
	"name":           "name",
	"expiry_date":    "expiry_date",
	"purchase_price": "purchase_price",
	"quantity":       "quantity",
	"lot_number":     "lot_number",
	"created_at":     "created_at",
	"updated_at":     "updated_at",
}

func (r *PositionRepository) Create(ctx context.Context, p *position.Position) error {
	_, err := r.coll.InsertOne(ctx, toPositionDoc(p))
	return err
}

func (r *PositionRepository) GetByID(ctx context.Context, id string) (*position.Position, error) {
	var d positionDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

// Update persists descriptive fields only; quantity is intentionally excluded.
func (r *PositionRepository) Update(ctx context.Context, p *position.Position) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := r.coll.UpdateByID(ctx, p.ID, bson.M{"$set": bson.M{
		"name":           p.Name,
		"brand_id":       p.BrandID,
		"contract_name":  p.ContractName,
		"expiry_date":    p.ExpiryDate,
		"lot_number":     p.LotNumber,
		"purchase_price": p.PurchasePrice,
		"mass_grams":     p.MassGrams,
		"updated_at":     p.UpdatedAt,
	}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *PositionRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *PositionRepository) List(ctx context.Context, f position.Filter) ([]position.Position, int64, error) {
	filter := bson.M{}
	if f.Q != "" {
		rx := caseInsensitiveContains(f.Q)
		filter["$or"] = bson.A{
			bson.M{"name": rx},
			bson.M{"contract_name": rx},
			bson.M{"lot_number": rx},
		}
	}
	if f.BrandID != "" {
		filter["brand_id"] = f.BrandID
	}
	if f.InStock {
		filter["quantity"] = bson.M{"$gt": 0}
	}
	if f.ExpiryBefore != nil || f.ExpiryAfter != nil {
		expiry := bson.M{}
		if f.ExpiryAfter != nil {
			expiry["$gte"] = *f.ExpiryAfter
		}
		if f.ExpiryBefore != nil {
			expiry["$lte"] = *f.ExpiryBefore
		}
		filter["expiry_date"] = expiry
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, positionSortFields, "created_at"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []positionDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	positions := make([]position.Position, len(docs))
	for i, d := range docs {
		positions[i] = *d.toDomain()
	}
	return positions, total, nil
}

func (r *PositionRepository) CountByBrand(ctx context.Context, brandID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"brand_id": brandID})
}

func (r *PositionRepository) IncrementQuantity(ctx context.Context, id string, delta int) error {
	res, err := r.coll.UpdateByID(ctx, id, bson.M{
		"$inc": bson.M{"quantity": delta},
		"$set": bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

// DecrementIfAvailable atomically subtracts qty only when stock is sufficient.
// It reports ok=false (no error) when the position exists but stock is too low,
// and returns store.ErrorNotFound when the position does not exist — the no-match
// result of the conditional update is disambiguated with a follow-up existence
// check (only on the no-match path, so the happy path pays nothing).
func (r *PositionRepository) DecrementIfAvailable(ctx context.Context, id string, qty int) (bool, error) {
	res := r.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": id, "quantity": bson.M{"$gte": qty}},
		bson.M{
			"$inc": bson.M{"quantity": -qty},
			"$set": bson.M{"updated_at": time.Now().UTC()},
		},
	)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			n, cerr := r.coll.CountDocuments(ctx, bson.M{"_id": id})
			if cerr != nil {
				return false, cerr
			}
			if n == 0 {
				return false, store.ErrorNotFound
			}
			return false, nil // exists but stock too low
		}
		return false, err
	}
	return true, nil
}
