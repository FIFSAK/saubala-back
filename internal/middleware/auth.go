package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/pkg/auth"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

type contextKey string

const userContextKey contextKey = "current_user"

// Authenticator validates the Bearer access token, loads the user, ensures the
// account is active and stores the user in the request context.
func Authenticator(tm *auth.TokenManager, users user.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				web.WriteError(w, web.Unauthorized("отсутствует или некорректный заголовок авторизации"))
				return
			}

			claims, err := tm.Parse(token)
			if err != nil {
				web.WriteError(w, web.Unauthorized("недействительный или истёкший токен"))
				return
			}

			u, err := users.GetByID(r.Context(), claims.UserID)
			if err != nil {
				web.WriteError(w, web.Unauthorized("пользователь больше не существует"))
				return
			}
			if !u.IsActive {
				web.WriteError(w, web.Forbidden("учётная запись деактивирована"))
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin allows only admin-capable users (super_admin or admin) through.
// It must be mounted after Authenticator.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r.Context())
		if !ok {
			web.WriteError(w, web.Unauthorized("требуется аутентификация"))
			return
		}
		if !u.Role.IsAdmin() {
			web.WriteError(w, web.Forbidden("требуются права администратора"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CurrentUser returns the authenticated user stored in the context, if any.
func CurrentUser(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value(userContextKey).(*user.User)
	return u, ok
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
