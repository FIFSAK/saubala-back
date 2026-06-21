package user

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role enumerates the access levels of a user.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

// IsValid reports whether r is a known role.
func (r Role) IsValid() bool {
	switch r {
	case RoleSuperAdmin, RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}

// IsAdmin reports whether the role may manage users (super_admin or admin).
func (r Role) IsAdmin() bool {
	return r == RoleSuperAdmin || r == RoleAdmin
}

// IsAssignable reports whether the role can be assigned through the API.
// super_admin is created only via seeding and is never assignable.
func (r Role) IsAssignable() bool {
	return r == RoleAdmin || r == RoleUser
}

// MinPasswordLength is the minimum accepted plaintext password length.
const MinPasswordLength = 8

// User is an application user (an employee of the company).
type User struct {
	ID           string
	Email        string
	PasswordHash string
	FullName     string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// New constructs a validated user with a generated ID and timestamps. The
// password hash is set separately by the service layer after hashing.
func New(email, fullName string, role Role) (*User, error) {
	// Validate the raw input first so display-name / comment forms are rejected,
	// then store the canonical bare address.
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	email = NormalizeEmail(email)
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role %q", role)
	}

	now := time.Now().UTC()
	return &User{
		ID:        uuid.NewString(),
		Email:     email,
		FullName:  strings.TrimSpace(fullName),
		Role:      role,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// NormalizeEmail reduces an email to its canonical bare address (stripping any
// display name or comment) and lower-cases it for consistent storage and lookup.
func NormalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	if addr, err := mail.ParseAddress(email); err == nil {
		email = addr.Address
	}
	return strings.ToLower(email)
}

// ValidateEmail checks that email is a plain, non-empty address. Display-name
// forms ("Name <a@b.kz>"), comments and quoting are rejected so that the stored
// value and the unique index always operate on canonical bare addresses.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email address")
	}
	if !strings.EqualFold(addr.Address, email) {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

// ValidatePassword enforces the minimum password policy.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	return nil
}
