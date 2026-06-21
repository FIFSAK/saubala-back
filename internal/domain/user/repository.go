package user

import "context"

// Filter describes the query parameters accepted by the users list endpoint.
type Filter struct {
	Q        string // matched against email / full_name (case-insensitive)
	Role     Role   // empty = any role
	IsActive *bool  // nil = any
	Page     int
	PageSize int
	Sort     string
	Order    string
}

// Repository is the persistence contract for users.
type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	List(ctx context.Context, f Filter) ([]User, int64, error)
	// CountActiveAdmins counts active admin-capable users (admin or super_admin),
	// optionally excluding the user with excludeID. Used to prevent locking the
	// system out of its last administrator.
	CountActiveAdmins(ctx context.Context, excludeID string) (int64, error)
}
