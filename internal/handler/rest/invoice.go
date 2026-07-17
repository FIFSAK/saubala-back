package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	invoicesvc "github.com/FIFSAK/saubala-back/internal/service/invoice"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// InvoiceHandler exposes stateless release-waybill (Форма З-2) generation.
type InvoiceHandler struct {
	invoices *invoicesvc.Service
}

func NewInvoiceHandler(invoices *invoicesvc.Service) *InvoiceHandler {
	return &InvoiceHandler{invoices: invoices}
}

func (h *InvoiceHandler) Register(r chi.Router) {
	r.Post("/releases/{id}/invoice", h.Generate)
}

// generateInvoiceRequest carries optional header overrides; empty fields fall
// back to the data stored on the release at creation time.
type generateInvoiceRequest struct {
	DocumentNumber   string    `json:"document_number"`
	DocumentDate     time.Time `json:"document_date"`
	RecipientName    string    `json:"recipient_name"`
	RecipientAddress string    `json:"recipient_address"`
}

// Generate builds the waybill for a release and streams it back as an XLSX or PDF
// download. The format is chosen via ?format=xlsx|pdf (default xlsx). The body is
// optional — the header data stored on the release is used by default.
func (h *InvoiceHandler) Generate(w http.ResponseWriter, r *http.Request) {
	format := invoicesvc.Format(r.URL.Query().Get("format"))
	if format == "" {
		format = invoicesvc.FormatXLSX
	}

	var req generateInvoiceRequest
	if r.ContentLength != 0 {
		if err := web.Decode(r, &req); err != nil {
			web.WriteError(w, err)
			return
		}
	}

	out, err := h.invoices.Generate(r.Context(), invoicesvc.GenerateInput{
		ReleaseID:        chi.URLParam(r, "id"),
		Format:           format,
		DocumentNumber:   req.DocumentNumber,
		DocumentDate:     req.DocumentDate,
		RecipientName:    req.RecipientName,
		RecipientAddress: req.RecipientAddress,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", out.ContentType)
	w.Header().Set("Content-Disposition", contentDisposition(out.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(out.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Data)
}

// contentDisposition builds an attachment header with an RFC 5987 UTF-8 filename
// so Cyrillic names survive (with an ASCII fallback for old clients).
func contentDisposition(name string) string {
	return "attachment; filename=\"invoice\"; filename*=UTF-8''" + rfc5987Escape(name)
}

// rfc5987Escape percent-encodes a string per RFC 5987 (attr-char set kept literal).
func rfc5987Escape(s string) string {
	const upperhex = "0123456789ABCDEF"
	isAttrChar := func(b byte) bool {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
			return true
		}
		switch b {
		case '!', '#', '$', '&', '+', '-', '.', '^', '_', '`', '|', '~':
			return true
		}
		return false
	}
	out := make([]byte, 0, len(s)*3)
	for i := 0; i < len(s); i++ {
		b := s[i]
		if isAttrChar(b) {
			out = append(out, b)
			continue
		}
		out = append(out, '%', upperhex[b>>4], upperhex[b&0x0f])
	}
	return string(out)
}
