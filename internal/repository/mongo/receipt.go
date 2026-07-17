package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/receipt"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type receiptLineDoc struct {
	PositionID string `bson:"position_id"`
	Quantity   int    `bson:"quantity"`
}

type receiptDoc struct {
	ID            string           `bson:"_id"`
	Date          time.Time        `bson:"date"`
	Note          string           `bson:"note"`
	SupplierID    string           `bson:"supplier_id,omitempty"`
	Counterparty  string           `bson:"counterparty,omitempty"`
	InvoiceAmount int64            `bson:"invoice_amount,omitempty"`
	Lines         []receiptLineDoc `bson:"lines"`
	CreatedBy     string           `bson:"created_by"`
	CreatedAt     time.Time        `bson:"created_at"`
}

func toReceiptDoc(r *receipt.Receipt) receiptDoc {
	lines := make([]receiptLineDoc, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = receiptLineDoc{PositionID: l.PositionID, Quantity: l.Quantity}
	}
	return receiptDoc{
		ID:            r.ID,
		Date:          r.Date,
		Note:          r.Note,
		SupplierID:    r.SupplierID,
		Counterparty:  r.Counterparty,
		InvoiceAmount: r.InvoiceAmount,
		Lines:         lines,
		CreatedBy:     r.CreatedBy,
		CreatedAt:     r.CreatedAt,
	}
}

func (d receiptDoc) toDomain() *receipt.Receipt {
	lines := make([]receipt.Line, len(d.Lines))
	for i, l := range d.Lines {
		lines[i] = receipt.Line{PositionID: l.PositionID, Quantity: l.Quantity}
	}
	return &receipt.Receipt{
		ID:            d.ID,
		Date:          d.Date,
		Note:          d.Note,
		SupplierID:    d.SupplierID,
		Counterparty:  d.Counterparty,
		InvoiceAmount: d.InvoiceAmount,
		Lines:         lines,
		CreatedBy:     d.CreatedBy,
		CreatedAt:     d.CreatedAt,
	}
}

// ReceiptRepository is the MongoDB implementation of receipt.Repository.
type ReceiptRepository struct {
	coll *mongo.Collection
}

func NewReceiptRepository(db *mongo.Database) *ReceiptRepository {
	return &ReceiptRepository{coll: db.Collection(collReceipts)}
}

var receiptSortFields = map[string]string{
	"date":       "date",
	"created_at": "created_at",
}

func (r *ReceiptRepository) Create(ctx context.Context, rec *receipt.Receipt) error {
	_, err := r.coll.InsertOne(ctx, toReceiptDoc(rec))
	return err
}

func (r *ReceiptRepository) GetByID(ctx context.Context, id string) (*receipt.Receipt, error) {
	var d receiptDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *ReceiptRepository) List(ctx context.Context, f receipt.Filter) ([]receipt.Receipt, int64, error) {
	filter := bson.M{}
	if f.PositionID != "" {
		filter["lines.position_id"] = f.PositionID
	}
	if f.SupplierID != "" {
		filter["supplier_id"] = f.SupplierID
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
		SetSort(sortDoc(f.Sort, f.Order, receiptSortFields, "date"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []receiptDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	receipts := make([]receipt.Receipt, len(docs))
	for i, d := range docs {
		receipts[i] = *d.toDomain()
	}
	return receipts, total, nil
}

func (r *ReceiptRepository) ListByPosition(ctx context.Context, positionID string) ([]receipt.Receipt, error) {
	cur, err := r.coll.Find(ctx, bson.M{"lines.position_id": positionID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []receiptDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	receipts := make([]receipt.Receipt, len(docs))
	for i, d := range docs {
		receipts[i] = *d.toDomain()
	}
	return receipts, nil
}

func (r *ReceiptRepository) CountBySupplier(ctx context.Context, supplierID string) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"supplier_id": supplierID})
}

func (r *ReceiptRepository) InvoiceTotalBySupplier(ctx context.Context, supplierIDs []string) (map[string]int64, error) {
	if len(supplierIDs) == 0 {
		return map[string]int64{}, nil
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"supplier_id": bson.M{"$in": supplierIDs}}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":   "$supplier_id",
			"total": bson.M{"$sum": "$invoice_amount"},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		ID    string `bson:"_id"`
		Total int64  `bson:"total"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Total
	}
	return out, nil
}
