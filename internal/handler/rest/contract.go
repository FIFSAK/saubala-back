package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/FIFSAK/saubala-back/internal/domain/contract"
	"github.com/FIFSAK/saubala-back/internal/middleware"
	contractsvc "github.com/FIFSAK/saubala-back/internal/service/contract"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// ContractHandler exposes the contracts (yearly plans) endpoints.
type ContractHandler struct {
	contracts *contractsvc.Service
}

func NewContractHandler(contracts *contractsvc.Service) *ContractHandler {
	return &ContractHandler{contracts: contracts}
}

func (h *ContractHandler) Register(r chi.Router) {
	r.Get("/contracts", h.List)
	r.Post("/contracts", h.Create)
	r.Get("/contracts/{id}", h.Get)
	r.Patch("/contracts/{id}", h.Update)
	r.Delete("/contracts/{id}", h.Delete)
}

// contractLineResponse includes per-line plan progress (used for detail views).
// ContractName falls back to the position-level «наименование по договору» for
// lines that predate line-level contract names.
type contractLineResponse struct {
	ID               string `json:"id"`
	PositionID       string `json:"position_id"`
	PositionName     string `json:"position_name"`
	LotNumber        string `json:"lot_number"`
	ContractName     string `json:"contract_name"`
	NTIN             string `json:"ntin"`
	PlannedQuantity  int    `json:"planned_quantity"`
	PlannedPrice     *int64 `json:"planned_price"`
	ReleasedQuantity int    `json:"released_quantity"`
	Remaining        int    `json:"remaining"`
}

// contractLinePlan is the plan-only line shape used in list responses.
type contractLinePlan struct {
	ID              string `json:"id"`
	PositionID      string `json:"position_id"`
	PositionName    string `json:"position_name"`
	LotNumber       string `json:"lot_number"`
	ContractName    string `json:"contract_name"`
	NTIN            string `json:"ntin"`
	PlannedQuantity int    `json:"planned_quantity"`
	PlannedPrice    *int64 `json:"planned_price"`
}

type contractResponse struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	CustomerOfficialName string                 `json:"customer_official_name"`
	CustomerAddress      string                 `json:"customer_address"`
	ContractNumber       string                 `json:"contract_number"`
	ContractDate         time.Time              `json:"contract_date"`
	BIN                  string                 `json:"bin"`
	Lines                []contractLineResponse `json:"lines"`
	TotalAmount          int64                  `json:"total_amount"`
	ReleasedAmount       int64                  `json:"released_amount"`
	CreatedBy            string                 `json:"created_by"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

type contractListItem struct {
	ID                   string             `json:"id"`
	Name                 string             `json:"name"`
	CustomerOfficialName string             `json:"customer_official_name"`
	CustomerAddress      string             `json:"customer_address"`
	ContractNumber       string             `json:"contract_number"`
	ContractDate         time.Time          `json:"contract_date"`
	BIN                  string             `json:"bin"`
	Lines                []contractLinePlan `json:"lines"`
	TotalAmount          int64              `json:"total_amount"`
	ReleasedAmount       int64              `json:"released_amount"`
	CreatedBy            string             `json:"created_by"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

// lineContractName resolves the per-contract product name of a line, falling
// back to the position-level value for contracts saved before the move.
func lineContractName(l domain.Line, pref contractsvc.PositionRef) string {
	if l.ContractName != "" {
		return l.ContractName
	}
	return pref.ContractName
}

func toContractResponse(c *domain.Contract, progress map[string]contractsvc.LineProgress, prefs map[string]contractsvc.PositionRef, amounts contractsvc.Amounts) contractResponse {
	lines := make([]contractLineResponse, len(c.Lines))
	for i, l := range c.Lines {
		released, remaining := 0, l.PlannedQuantity
		if p, ok := progress[l.ID]; ok {
			released, remaining = p.Released, p.Remaining
		}
		pref := prefs[l.PositionID]
		lines[i] = contractLineResponse{
			ID:               l.ID,
			PositionID:       l.PositionID,
			PositionName:     pref.Name,
			LotNumber:        pref.LotNumber,
			ContractName:     lineContractName(l, pref),
			NTIN:             l.NTIN,
			PlannedQuantity:  l.PlannedQuantity,
			PlannedPrice:     l.PlannedPrice,
			ReleasedQuantity: released,
			Remaining:        remaining,
		}
	}
	return contractResponse{
		ID:                   c.ID,
		Name:                 c.Name,
		CustomerOfficialName: c.CustomerOfficialName,
		CustomerAddress:      c.CustomerAddress,
		ContractNumber:       c.ContractNumber,
		ContractDate:         c.ContractDate,
		BIN:                  c.BIN,
		Lines:                lines,
		TotalAmount:          amounts.Total,
		ReleasedAmount:       amounts.Released,
		CreatedBy:            c.CreatedBy,
		CreatedAt:            c.CreatedAt,
		UpdatedAt:            c.UpdatedAt,
	}
}

func toContractListItem(c *domain.Contract, prefs map[string]contractsvc.PositionRef, amounts contractsvc.Amounts) contractListItem {
	lines := make([]contractLinePlan, len(c.Lines))
	for i, l := range c.Lines {
		pref := prefs[l.PositionID]
		lines[i] = contractLinePlan{
			ID:              l.ID,
			PositionID:      l.PositionID,
			PositionName:    pref.Name,
			LotNumber:       pref.LotNumber,
			ContractName:    lineContractName(l, pref),
			NTIN:            l.NTIN,
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return contractListItem{
		ID:                   c.ID,
		Name:                 c.Name,
		CustomerOfficialName: c.CustomerOfficialName,
		CustomerAddress:      c.CustomerAddress,
		ContractNumber:       c.ContractNumber,
		ContractDate:         c.ContractDate,
		BIN:                  c.BIN,
		Lines:                lines,
		TotalAmount:          amounts.Total,
		ReleasedAmount:       amounts.Released,
		CreatedBy:            c.CreatedBy,
		CreatedAt:            c.CreatedAt,
		UpdatedAt:            c.UpdatedAt,
	}
}

type contractLineRequest struct {
	ID              string `json:"id"`
	PositionID      string `json:"position_id"`
	ContractName    string `json:"contract_name"`
	NTIN            string `json:"ntin"`
	PlannedQuantity int    `json:"planned_quantity"`
	PlannedPrice    *int64 `json:"planned_price"`
}

func toServiceLines(in []contractLineRequest) []contractsvc.LineInput {
	lines := make([]contractsvc.LineInput, len(in))
	for i, l := range in {
		lines[i] = contractsvc.LineInput{
			ID:              l.ID,
			PositionID:      l.PositionID,
			ContractName:    l.ContractName,
			NTIN:            l.NTIN,
			PlannedQuantity: l.PlannedQuantity,
			PlannedPrice:    l.PlannedPrice,
		}
	}
	return lines
}

type createContractRequest struct {
	Name                 string                `json:"name"`
	CustomerOfficialName string                `json:"customer_official_name"`
	CustomerAddress      string                `json:"customer_address"`
	ContractNumber       string                `json:"contract_number"`
	ContractDate         time.Time             `json:"contract_date"`
	BIN                  string                `json:"bin"`
	Lines                []contractLineRequest `json:"lines"`
}

type updateContractRequest struct {
	Name                 *string                `json:"name"`
	CustomerOfficialName *string                `json:"customer_official_name"`
	CustomerAddress      *string                `json:"customer_address"`
	ContractNumber       *string                `json:"contract_number"`
	ContractDate         *time.Time             `json:"contract_date"`
	BIN                  *string                `json:"bin"`
	Lines                *[]contractLineRequest `json:"lines"`
}

func (h *ContractHandler) List(w http.ResponseWriter, r *http.Request) {
	p := web.ParseListParams(r)
	from, err := queryTimePtr(r, "date_from")
	if err != nil {
		web.WriteError(w, err)
		return
	}
	to, err := queryTimePtr(r, "date_to")
	if err != nil {
		web.WriteError(w, err)
		return
	}

	contracts, total, err := h.contracts.List(r.Context(), domain.Filter{
		Q:        p.Q,
		BIN:      r.URL.Query().Get("bin"),
		DateFrom: from,
		DateTo:   to,
		Page:     p.Page,
		PageSize: p.PageSize,
		Sort:     p.Sort,
		Order:    p.Order,
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	prefs, err := h.contracts.PositionRefs(r.Context(), contracts)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	amounts, err := h.contracts.AmountsFor(r.Context(), contracts)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	items := make([]contractListItem, len(contracts))
	for i := range contracts {
		items[i] = toContractListItem(&contracts[i], prefs, amounts[contracts[i].ID])
	}
	web.List(w, items, total, p)
}

func (h *ContractHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createContractRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}
	actor, _ := middleware.CurrentUser(r.Context())

	c, err := h.contracts.Create(r.Context(), contractsvc.CreateInput{
		Name:                 req.Name,
		CustomerOfficialName: req.CustomerOfficialName,
		CustomerAddress:      req.CustomerAddress,
		ContractNumber:       req.ContractNumber,
		ContractDate:         req.ContractDate,
		BIN:                  req.BIN,
		Lines:                toServiceLines(req.Lines),
		CreatedBy:            actorID(actor),
	})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	prefs, err := h.contracts.PositionRefs(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	amounts, err := h.contracts.AmountsFor(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	// A brand-new contract has no releases yet, so progress is all-zero.
	web.JSON(w, http.StatusCreated, toContractResponse(c, nil, prefs, amounts[c.ID]))
}

func (h *ContractHandler) Get(w http.ResponseWriter, r *http.Request) {
	c, progress, err := h.contracts.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		web.WriteError(w, err)
		return
	}
	prefs, err := h.contracts.PositionRefs(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	amounts, err := h.contracts.AmountsFor(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toContractResponse(c, progress, prefs, amounts[c.ID]))
}

func (h *ContractHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateContractRequest
	if err := web.Decode(r, &req); err != nil {
		web.WriteError(w, err)
		return
	}

	in := contractsvc.UpdateInput{
		Name:                 req.Name,
		CustomerOfficialName: req.CustomerOfficialName,
		CustomerAddress:      req.CustomerAddress,
		ContractNumber:       req.ContractNumber,
		ContractDate:         req.ContractDate,
		BIN:                  req.BIN,
	}
	if req.Lines != nil {
		lines := toServiceLines(*req.Lines)
		in.Lines = &lines
	}

	c, progress, err := h.contracts.Update(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		web.WriteError(w, err)
		return
	}
	prefs, err := h.contracts.PositionRefs(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	amounts, err := h.contracts.AmountsFor(r.Context(), []domain.Contract{*c})
	if err != nil {
		web.WriteError(w, err)
		return
	}
	web.JSON(w, http.StatusOK, toContractResponse(c, progress, prefs, amounts[c.ID]))
}

func (h *ContractHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.contracts.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		web.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
