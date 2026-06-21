package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type contractLineDoc struct {
	ID              string `bson:"id"`
	PositionID      string `bson:"position_id"`
	PlannedQuantity int    `bson:"planned_quantity"`
	PlannedPrice    *int64 `bson:"planned_price"`
}

type contractDoc struct {
	ID              string            `bson:"_id"`
	Name            string            `bson:"name"`
	CustomerAddress string            `bson:"customer_address"`
	ContractNumber  string            `bson:"contract_number"`
	ContractDate    time.Time         `bson:"contract_date"`
	BIN             string            `bson:"bin"`
	Lines           []contractLineDoc `bson:"lines"`
	CreatedBy       string            `bson:"created_by"`
	CreatedAt       time.Time         `bson:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at"`
}

func toContractDoc(c *contract.Contract) contractDoc {
	lines := make([]contractLineDoc, len(c.Lines))
	for i, l := range c.Lines {
		lines[i] = contractLineDoc{
			ID:              l.ID,
			PositionID:      l.PositionID,
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return contractDoc{
		ID:              c.ID,
		Name:            c.Name,
		CustomerAddress: c.CustomerAddress,
		ContractNumber:  c.ContractNumber,
		ContractDate:    c.ContractDate,
		BIN:             c.BIN,
		Lines:           lines,
		CreatedBy:       c.CreatedBy,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func (d contractDoc) toDomain() *contract.Contract {
	lines := make([]contract.Line, len(d.Lines))
	for i, l := range d.Lines {
		lines[i] = contract.Line{
			ID:              l.ID,
			PositionID:      l.PositionID,
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return &contract.Contract{
		ID:              d.ID,
		Name:            d.Name,
		CustomerAddress: d.CustomerAddress,
		ContractNumber:  d.ContractNumber,
		ContractDate:    d.ContractDate,
		BIN:             d.BIN,
		Lines:           lines,
		CreatedBy:       d.CreatedBy,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

// ContractRepository is the MongoDB implementation of contract.Repository.
type ContractRepository struct {
	coll *mongo.Collection
}

func NewContractRepository(db *mongo.Database) *ContractRepository {
	return &ContractRepository{coll: db.Collection(collContracts)}
}

var contractSortFields = map[string]string{
	"name":            "name",
	"contract_number": "contract_number",
	"contract_date":   "contract_date",
	"created_at":      "created_at",
	"updated_at":      "updated_at",
}

func (r *ContractRepository) Create(ctx context.Context, c *contract.Contract) error {
	_, err := r.coll.InsertOne(ctx, toContractDoc(c))
	if store.IsDuplicateKey(err) {
		return store.ErrDuplicate
	}
	return err
}

func (r *ContractRepository) GetByID(ctx context.Context, id string) (*contract.Contract, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

func (r *ContractRepository) GetByNumber(ctx context.Context, number string) (*contract.Contract, error) {
	return r.findOne(ctx, bson.M{"contract_number": number})
}

func (r *ContractRepository) findOne(ctx context.Context, filter bson.M) (*contract.Contract, error) {
	var d contractDoc
	if err := r.coll.FindOne(ctx, filter).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *ContractRepository) Update(ctx context.Context, c *contract.Contract) error {
	c.UpdatedAt = time.Now().UTC()
	doc := toContractDoc(c)
	res, err := r.coll.UpdateByID(ctx, c.ID, bson.M{"$set": bson.M{
		"name":             doc.Name,
		"customer_address": doc.CustomerAddress,
		"contract_number":  doc.ContractNumber,
		"contract_date":    doc.ContractDate,
		"bin":              doc.BIN,
		"lines":            doc.Lines,
		"updated_at":       doc.UpdatedAt,
	}})
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

func (r *ContractRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return store.ErrorNotFound
	}
	return nil
}

func (r *ContractRepository) List(ctx context.Context, f contract.Filter) ([]contract.Contract, int64, error) {
	filter := bson.M{}
	if f.Q != "" {
		rx := caseInsensitiveContains(f.Q)
		filter["$or"] = bson.A{
			bson.M{"name": rx},
			bson.M{"contract_number": rx},
		}
	}
	if f.BIN != "" {
		filter["bin"] = f.BIN
	}
	if f.DateFrom != nil || f.DateTo != nil {
		date := bson.M{}
		if f.DateFrom != nil {
			date["$gte"] = *f.DateFrom
		}
		if f.DateTo != nil {
			date["$lte"] = *f.DateTo
		}
		filter["contract_date"] = date
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, contractSortFields, "created_at"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []contractDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	contracts := make([]contract.Contract, len(docs))
	for i, d := range docs {
		contracts[i] = *d.toDomain()
	}
	return contracts, total, nil
}

func (r *ContractRepository) CountByPosition(ctx context.Context, positionID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"lines.position_id": positionID})
}
