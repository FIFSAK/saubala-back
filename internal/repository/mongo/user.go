package mongo

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/pkg/store"
)

type userDoc struct {
	ID           string    `bson:"_id"`
	Email        string    `bson:"email"`
	PasswordHash string    `bson:"password_hash"`
	FullName     string    `bson:"full_name"`
	Role         string    `bson:"role"`
	IsActive     bool      `bson:"is_active"`
	CreatedAt    time.Time `bson:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

func toUserDoc(u *user.User) userDoc {
	return userDoc{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Role:         string(u.Role),
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func (d userDoc) toDomain() *user.User {
	return &user.User{
		ID:           d.ID,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		FullName:     d.FullName,
		Role:         user.Role(d.Role),
		IsActive:     d.IsActive,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

// UserRepository is the MongoDB implementation of user.Repository.
type UserRepository struct {
	coll *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection(collUsers)}
}

var userSortFields = map[string]string{
	"email":      "email",
	"full_name":  "full_name",
	"role":       "role",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	_, err := r.coll.InsertOne(ctx, toUserDoc(u))
	if store.IsDuplicateKey(err) {
		return store.ErrDuplicate
	}
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return r.findOne(ctx, bson.M{"email": user.NormalizeEmail(email)})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.M) (*user.User, error) {
	var d userDoc
	if err := r.coll.FindOne(ctx, filter).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, store.ErrorNotFound
		}
		return nil, err
	}
	return d.toDomain(), nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	u.UpdatedAt = time.Now().UTC()
	res, err := r.coll.UpdateByID(ctx, u.ID, bson.M{"$set": bson.M{
		"email":         u.Email,
		"password_hash": u.PasswordHash,
		"full_name":     u.FullName,
		"role":          string(u.Role),
		"is_active":     u.IsActive,
		"updated_at":    u.UpdatedAt,
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

func (r *UserRepository) List(ctx context.Context, f user.Filter) ([]user.User, int64, error) {
	filter := bson.M{}
	if f.Q != "" {
		rx := caseInsensitiveContains(f.Q)
		filter["$or"] = bson.A{
			bson.M{"email": rx},
			bson.M{"full_name": rx},
		}
	}
	if f.Role != "" {
		filter["role"] = string(f.Role)
	}
	if f.IsActive != nil {
		filter["is_active"] = *f.IsActive
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip, limit := paginate(f.Page, f.PageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sortDoc(f.Sort, f.Order, userSortFields, "created_at"))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []userDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	users := make([]user.User, len(docs))
	for i, d := range docs {
		users[i] = *d.toDomain()
	}
	return users, total, nil
}

func (r *UserRepository) CountActiveAdmins(ctx context.Context, excludeID string) (int64, error) {
	filter := bson.M{
		"is_active": true,
		"role":      bson.M{"$in": bson.A{string(user.RoleAdmin), string(user.RoleSuperAdmin)}},
	}
	if excludeID != "" {
		filter["_id"] = bson.M{"$ne": excludeID}
	}
	return r.coll.CountDocuments(ctx, filter)
}
