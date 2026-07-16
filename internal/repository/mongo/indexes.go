package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates all required indexes programmatically. It is idempotent
// and safe to run on every startup (replaces the migration step).
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	specs := map[string][]mongo.IndexModel{
		collUsers: {
			{
				Keys:    bson.D{{Key: "email", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_email"),
			},
		},
		collBrands: {
			{
				// unique brand name, but only among non-deleted brands
				Keys: bson.D{{Key: "name", Value: 1}},
				Options: options.Index().
					SetUnique(true).
					SetName("uniq_name_active").
					SetPartialFilterExpression(bson.M{"deleted_at": bson.M{"$eq": nil}}),
			},
		},
		collPositions: {
			{Keys: bson.D{{Key: "brand_id", Value: 1}}, Options: options.Index().SetName("idx_brand_id")},
			{Keys: bson.D{{Key: "lot_number", Value: 1}}, Options: options.Index().SetName("idx_lot_number")},
			{Keys: bson.D{{Key: "expiry_date", Value: 1}}, Options: options.Index().SetName("idx_expiry_date")},
		},
		collContracts: {
			{
				Keys:    bson.D{{Key: "contract_number", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_contract_number"),
			},
			{Keys: bson.D{{Key: "bin", Value: 1}}, Options: options.Index().SetName("idx_bin")},
			{Keys: bson.D{{Key: "contract_date", Value: 1}}, Options: options.Index().SetName("idx_contract_date")},
		},
		collReceipts: {
			{Keys: bson.D{{Key: "lines.position_id", Value: 1}}, Options: options.Index().SetName("idx_lines_position_id")},
			{Keys: bson.D{{Key: "date", Value: 1}}, Options: options.Index().SetName("idx_date")},
		},
		collReleases: {
			{Keys: bson.D{{Key: "contract_id", Value: 1}}, Options: options.Index().SetName("idx_contract_id")},
			{Keys: bson.D{{Key: "date", Value: 1}}, Options: options.Index().SetName("idx_date")},
		},
		collAdjustments: {
			{Keys: bson.D{{Key: "position_id", Value: 1}}, Options: options.Index().SetName("idx_position_id")},
			{Keys: bson.D{{Key: "created_at", Value: 1}}, Options: options.Index().SetName("idx_created_at")},
		},
	}

	for coll, models := range specs {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("create indexes for %s: %w", coll, err)
		}
	}
	return nil
}
