package mongo

import (
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Collection names.
const (
	collUsers     = "users"
	collBrands    = "brands"
	collPositions = "positions"
	collReceipts  = "receipts"
	collContracts = "contracts"
	collReleases  = "releases"

	collAdjustments = "adjustments"

	collSettings      = "settings"
	collOrganizations = "organizations"
	collSuppliers     = "suppliers"
)

// caseInsensitiveContains builds a case-insensitive "contains" regex condition
// for q, escaping any regex metacharacters in the user input.
func caseInsensitiveContains(q string) primitive.Regex {
	return primitive.Regex{Pattern: regexp.QuoteMeta(q), Options: "i"}
}

// sortDoc builds a sort specification. sortField is matched against the allowed
// map (API field name -> bson field name); unknown or empty fields fall back to
// def. order "asc" sorts ascending, anything else descending.
func sortDoc(sortField, order string, allowed map[string]string, def string) bson.D {
	field := def
	if mapped, ok := allowed[sortField]; ok {
		field = mapped
	}
	dir := -1
	if order == "asc" {
		dir = 1
	}
	return bson.D{{Key: field, Value: dir}}
}

// paginate returns the skip/limit for a 1-based page and page size.
func paginate(page, pageSize int) (skip, limit int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return int64((page - 1) * pageSize), int64(pageSize)
}
