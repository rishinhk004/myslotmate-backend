package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PayoutController struct {
	payoutService service.PayoutService
}

func NewPayoutController(s service.PayoutService) *PayoutController {
	return &PayoutController{payoutService: s}
}

func (c *PayoutController) RegisterRoutes(r chi.Router) {
	r.Route("/payouts", func(r chi.Router) {
		// Payout Methods
		r.Post("/methods", c.AddPayoutMethod)
		r.Get("/methods/{hostID}", c.ListPayoutMethods)
		r.Put("/methods/{methodID}/primary", c.SetPrimaryMethod)
		r.Delete("/methods/{methodID}", c.DeletePayoutMethod)

		// Withdrawals
		r.Post("/withdraw", c.RequestWithdrawal)

		// Earnings
		r.Get("/earnings/{hostID}", c.GetEarningsSummary)

		// Payout History
		r.Get("/history/{hostID}", c.GetPayoutHistory)
	})
}

// ── Payout Methods ──────────────────────────────────────────────────────────

type AddPayoutMethodReq struct {
	HostID          uuid.UUID               `json:"host_id"`
	Type            models.PayoutMethodType `json:"type"`
	BankName        *string                 `json:"bank_name,omitempty"`
	AccountType     *string                 `json:"account_type,omitempty"`
	AccountNumber   *string                 `json:"account_number,omitempty"`
	IFSC            *string                 `json:"ifsc,omitempty"`
	BeneficiaryName *string                 `json:"beneficiary_name,omitempty"`
	UPIID           *string                 `json:"upi_id,omitempty"`
}

func (c *PayoutController) AddPayoutMethod(w http.ResponseWriter, r *http.Request) {
	var req AddPayoutMethodReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.AddPayoutMethodRequest{
		Type:            req.Type,
		BankName:        req.BankName,
		AccountType:     req.AccountType,
		AccountNumber:   req.AccountNumber,
		IFSC:            req.IFSC,
		BeneficiaryName: req.BeneficiaryName,
		UPIID:           req.UPIID,
	}

	pm, err := c.payoutService.AddPayoutMethod(r.Context(), req.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, pm)
}

func (c *PayoutController) ListPayoutMethods(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	methods, err := c.payoutService.ListPayoutMethods(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, methods)
}

func (c *PayoutController) SetPrimaryMethod(w http.ResponseWriter, r *http.Request) {
	methodID, err := uuid.Parse(chi.URLParam(r, "methodID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid method ID")
		return
	}

	var body struct {
		HostID uuid.UUID `json:"host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := c.payoutService.SetPrimaryMethod(r.Context(), body.HostID, methodID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "primary method updated"})
}

func (c *PayoutController) DeletePayoutMethod(w http.ResponseWriter, r *http.Request) {
	methodID, err := uuid.Parse(chi.URLParam(r, "methodID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid method ID")
		return
	}

	// Host ID from query or body for authorization
	hostIDStr := r.URL.Query().Get("host_id")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Missing or invalid host_id query param")
		return
	}

	if err := c.payoutService.DeletePayoutMethod(r.Context(), hostID, methodID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "payout method deleted"})
}

// ── Withdrawal ──────────────────────────────────────────────────────────────

type WithdrawalReq struct {
	HostID         uuid.UUID  `json:"host_id"`
	AmountCents    int64      `json:"amount_cents"`
	PayoutMethodID *uuid.UUID `json:"payout_method_id,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
}

func (c *PayoutController) RequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req WithdrawalReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.WithdrawalRequest{
		AmountCents:    req.AmountCents,
		PayoutMethodID: req.PayoutMethodID,
		IdempotencyKey: req.IdempotencyKey,
	}

	payment, err := c.payoutService.RequestWithdrawal(r.Context(), req.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, payment)
}

// ── Earnings ────────────────────────────────────────────────────────────────

func (c *PayoutController) GetEarningsSummary(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	summary, err := c.payoutService.GetEarningsSummary(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, summary)
}

// ── Payout History ──────────────────────────────────────────────────────────

func (c *PayoutController) GetPayoutHistory(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	payments, err := c.payoutService.GetPayoutHistory(r.Context(), hostID, limit, offset)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, payments)
}
