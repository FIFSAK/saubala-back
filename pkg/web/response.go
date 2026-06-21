package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/FIFSAK/saubala-back/pkg/store"
)

// Error is an HTTP-aware error carrying the status code that should be returned
// to the client. Service-layer code returns *Error so handlers stay thin and the
// exact status semantics (400/403/404/409/422 ...) live next to the business rule.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func NewError(status int, msg string) *Error { return &Error{Status: status, Message: msg} }

func BadRequest(msg string) *Error    { return NewError(http.StatusBadRequest, msg) }
func Unauthorized(msg string) *Error  { return NewError(http.StatusUnauthorized, msg) }
func Forbidden(msg string) *Error     { return NewError(http.StatusForbidden, msg) }
func NotFound(msg string) *Error      { return NewError(http.StatusNotFound, msg) }
func Conflict(msg string) *Error      { return NewError(http.StatusConflict, msg) }
func Unprocessable(msg string) *Error { return NewError(http.StatusUnprocessableEntity, msg) }
func Internal(msg string) *Error      { return NewError(http.StatusInternalServerError, msg) }

// errorBody is the single error envelope used across the API.
type errorBody struct {
	Error string `json:"error"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError maps an error to the appropriate HTTP status and writes the
// standard error envelope. *Error values carry their own status; known store
// sentinels are mapped; anything else becomes a 500 without leaking internals.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		JSON(w, apiErr.Status, errorBody{Error: apiErr.Message})
		return
	}
	if errors.Is(err, store.ErrorNotFound) {
		JSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	JSON(w, http.StatusInternalServerError, errorBody{Error: "internal server error"})
}

// Decode reads a JSON request body into v, returning a 400 *Error on malformed input.
func Decode(r *http.Request, v any) error {
	if r.Body == nil {
		return BadRequest("empty request body")
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return BadRequest("invalid request body")
	}
	return nil
}
