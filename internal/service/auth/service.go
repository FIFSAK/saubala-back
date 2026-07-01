package authsvc

import (
	"context"
	"errors"

	"github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service handles authentication (login / token issuing).
type Service struct {
	users  user.Repository
	tokens *auth.TokenManager
}

func NewService(users user.Repository, tokens *auth.TokenManager) *Service {
	return &Service{users: users, tokens: tokens}
}

// Result is the outcome of a successful login.
type Result struct {
	Token string
	User  *user.User
}

// Login verifies credentials and issues an access token. Invalid credentials and
// unknown emails both return 401 (no user enumeration); deactivated accounts 403.
func (s *Service) Login(ctx context.Context, email, password string) (*Result, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.Unauthorized("неверный email или пароль")
		}
		return nil, err
	}

	if !auth.CheckPassword(u.PasswordHash, password) {
		return nil, web.Unauthorized("неверный email или пароль")
	}
	if !u.IsActive {
		return nil, web.Forbidden("учётная запись деактивирована")
	}

	token, err := s.tokens.Generate(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	return &Result{Token: token, User: u}, nil
}
