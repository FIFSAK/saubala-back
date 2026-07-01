package user

import (
	"context"
	"errors"

	domain "github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service implements user management with the protective rules from the spec.
type Service struct {
	users domain.Repository
}

func NewService(users domain.Repository) *Service {
	return &Service{users: users}
}

// CreateInput is the payload for creating a user.
type CreateInput struct {
	Email    string
	Password string
	FullName string
	Role     string
}

// UpdateInput carries the optionally-updated fields of a user.
type UpdateInput struct {
	FullName *string
	Role     *string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domain.User, error) {
	role := domain.Role(in.Role)
	if !role.IsAssignable() {
		return nil, web.BadRequest("роль должна быть «admin» или «user»")
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return nil, web.BadRequest(err.Error())
	}

	u, err := domain.New(in.Email, in.FullName, role)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = hash

	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, web.Conflict("пользователь с таким email уже существует")
		}
		return nil, err
	}
	return u, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "пользователь не найден")
	}
	return u, nil
}

func (s *Service) List(ctx context.Context, f domain.Filter) ([]domain.User, int64, error) {
	return s.users.List(ctx, f)
}

func (s *Service) Update(ctx context.Context, actor *domain.User, id string, in UpdateInput) (*domain.User, error) {
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "пользователь не найден")
	}

	if in.FullName != nil {
		target.FullName = *in.FullName
	}

	if in.Role != nil {
		newRole := domain.Role(*in.Role)
		if !newRole.IsAssignable() {
			return nil, web.BadRequest("роль должна быть «admin» или «user»")
		}
		if err := s.applyRoleChange(ctx, actor, target, newRole); err != nil {
			return nil, err
		}
	}

	if err := s.users.Update(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

// applyRoleChange validates and applies a role transition for target.
func (s *Service) applyRoleChange(ctx context.Context, actor, target *domain.User, newRole domain.Role) error {
	if target.Role == newRole {
		return nil
	}
	if target.Role == domain.RoleSuperAdmin {
		return web.Forbidden("роль супер-администратора нельзя изменить")
	}
	demoting := target.Role.IsAdmin() && !newRole.IsAdmin()
	if demoting {
		if actor != nil && actor.ID == target.ID {
			return web.Forbidden("вы не можете понизить свою роль")
		}
		if err := s.ensureNotLastAdmin(ctx, target.ID); err != nil {
			return err
		}
	}
	target.Role = newRole
	return nil
}

func (s *Service) Activate(ctx context.Context, id string) (*domain.User, error) {
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "пользователь не найден")
	}
	target.IsActive = true
	if err := s.users.Update(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

func (s *Service) Deactivate(ctx context.Context, actor *domain.User, id string) (*domain.User, error) {
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err, "пользователь не найден")
	}
	if target.Role == domain.RoleSuperAdmin {
		return nil, web.Forbidden("супер-администратора нельзя деактивировать")
	}
	if actor != nil && actor.ID == target.ID {
		return nil, web.Forbidden("вы не можете деактивировать себя")
	}
	if target.IsActive && target.Role.IsAdmin() {
		if err := s.ensureNotLastAdmin(ctx, target.ID); err != nil {
			return nil, err
		}
	}

	target.IsActive = false
	if err := s.users.Update(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

func (s *Service) ResetPassword(ctx context.Context, actor *domain.User, id, newPassword string) error {
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return mapNotFound(err, "пользователь не найден")
	}
	// Only the super administrator may reset the super administrator's password.
	if target.Role == domain.RoleSuperAdmin && (actor == nil || actor.Role != domain.RoleSuperAdmin) {
		return web.Forbidden("только супер-администратор может сбросить этот пароль")
	}
	if err := domain.ValidatePassword(newPassword); err != nil {
		return web.BadRequest(err.Error())
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	target.PasswordHash = hash
	return s.users.Update(ctx, target)
}

// EnsureSuperAdmin seeds the super administrator account if it does not exist.
func (s *Service) EnsureSuperAdmin(ctx context.Context, email, password string) error {
	_, err := s.users.GetByEmail(ctx, email)
	if err == nil {
		return nil // already present
	}
	if !errors.Is(err, store.ErrorNotFound) {
		return err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	u, err := domain.New(email, "Super Admin", domain.RoleSuperAdmin)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil // created concurrently
		}
		return err
	}
	return nil
}

func (s *Service) ensureNotLastAdmin(ctx context.Context, excludeID string) error {
	remaining, err := s.users.CountActiveAdmins(ctx, excludeID)
	if err != nil {
		return err
	}
	if remaining == 0 {
		return web.Conflict("нельзя удалить последнего администратора")
	}
	return nil
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, store.ErrorNotFound) {
		return web.NotFound(msg)
	}
	return err
}
