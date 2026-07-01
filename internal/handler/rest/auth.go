package rest

import (
	"net/http"

	"github.com/FIFSAK/saubala-back/internal/middleware"
	authsvc "github.com/FIFSAK/saubala-back/internal/service/auth"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// AuthHandler exposes authentication endpoints.
type AuthHandler struct {
	auth *authsvc.Service
}

func NewAuthHandler(auth *authsvc.Service) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

// Login authenticates a user and returns an access token. (POST /auth/login)
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}

	res, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		web.WriteError(w, err)
		return
	}

	web.JSON(w, http.StatusOK, loginResponse{
		AccessToken: res.Token,
		User:        toUserResponse(res.User),
	})
}

// Me returns the currently authenticated user. (GET /auth/me)
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.CurrentUser(r.Context())
	if !ok {
		web.WriteError(w, web.Unauthorized("требуется аутентификация"))
		return
	}
	web.JSON(w, http.StatusOK, toUserResponse(u))
}
