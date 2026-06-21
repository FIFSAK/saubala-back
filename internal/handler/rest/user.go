package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	usersvc "github.com/FIFSAK/saubala-back/internal/service/user"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// UserHandler exposes user-management endpoints (admin-only).
type UserHandler struct {
	users *usersvc.Service
}

func NewUserHandler(users *usersvc.Service) *UserHandler {
	return &UserHandler{users: users}
}

// Register mounts the user-management routes.
func (h *UserHandler) Register(r chi.Router) {
	r.Get("/users", h.List)
	r.Post("/users", h.Create)
	r.Get("/users/{id}", h.Get)
	r.Patch("/users/{id}", h.Update)
	r.Post("/users/{id}/activate", h.Activate)
	r.Post("/users/{id}/deactivate", h.Deactivate)
	r.Post("/users/{id}/reset-password", h.ResetPassword)
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      string(u.Role),
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)
	filter := domain.Filter{
		Q:        p.Q,
		Role:     domain.Role(r.URL.Query().Get("role")),
		IsActive: queryBoolPtr(r, "is_active"),
		Page:     p.Page,
		PageSize: p.PageSize,
		Sort:     p.Sort,
		Order:    p.Order,
	}

	users, total, err := h.users.List(r.Context(), filter)
	if err != nil {
		web.WriteError(w, err)
		return
	}

	items := make([]userResponse, len(users))
	for i := range users {
		items[i] = toUserResponse(&users[i])
	}
	web.List(w, items, total, p)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}

	u, err := h.users.Create(r.Context(), usersvc.CreateInput{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		Role:     req.Role,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusCreated, toUserResponse(u))
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toUserResponse(u))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	u, err := h.users.Update(r.Context(), actor, chi.URLParam(r, "id"), usersvc.UpdateInput{
		FullName: req.FullName,
		Role:     req.Role,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toUserResponse(u))
}

func (h *UserHandler) Activate(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.Activate(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toUserResponse(u))
}

func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	actor, _ := middleware.CurrentUser(r.Context())
	u, err := h.users.Deactivate(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toUserResponse(u))
}

func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	if err := h.users.ResetPassword(r.Context(), actor, chi.URLParam(r, "id"), req.NewPassword); err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
