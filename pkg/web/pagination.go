package web

import (
	"net/http"
	"strconv"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ListParams holds the common pagination / search / sort query parameters shared
// by every list endpoint. Entity-specific filters are parsed separately by each
// handler and combined with these values into the domain filter.
type ListParams struct {
	Page     int
	PageSize int
	Q        string
	Sort     string
	Order    string
}

// ParseListParams extracts the common list parameters from the request, applying
// defaults and clamping page_size to MaxPageSize.
func ParseListParams(r *http.Request) ListParams {
	q := r.URL.Query()
	p := ListParams{
		Page:     atoiDefault(q.Get("page"), DefaultPage),
		PageSize: atoiDefault(q.Get("page_size"), DefaultPageSize),
		Q:        q.Get("q"),
		Sort:     q.Get("sort"),
		Order:    q.Get("order"),
	}
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	if p.Order != "asc" && p.Order != "desc" {
		p.Order = "desc"
	}
	return p
}

// ListResponse is the standard envelope for paginated list endpoints.
type ListResponse struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// List writes a paginated list response.
func List(w http.ResponseWriter, items any, total int64, p ListParams) {
	JSON(w, http.StatusOK, ListResponse{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.PageSize,
	})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
