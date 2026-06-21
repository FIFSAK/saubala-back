package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/FIFSAK/saubala-back/internal/domain/user"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// actorID returns the id of the acting user, or "" when unauthenticated.
func actorID(u *user.User) string {
	if u == nil {
		return ""
	}
	return u.ID
}

// queryBoolPtr parses an optional boolean query parameter. Returns nil when the
// parameter is absent or unparseable.
func queryBoolPtr(r *http.Request, key string) *bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &v
}

// queryTimePtr parses an optional RFC3339 timestamp query parameter. A present
// but malformed value yields a 400 *web.Error.
func queryTimePtr(r *http.Request, key string) (*time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, web.BadRequest("invalid " + key + ": expected RFC3339 timestamp")
	}
	t = t.UTC()
	return &t, nil
}
