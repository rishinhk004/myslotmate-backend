package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/auth"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/service"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AdminController handles admin-only host-application management and platform payouts.
type AdminController struct {
	hostService   service.HostService
	payoutService service.PayoutService
	firebaseAuth  *fbauth.Client
	adminEmail    string
}

func NewAdminController(hs service.HostService, ps service.PayoutService, fa *fbauth.Client, adminEmail string) *AdminController {
	return &AdminController{
		hostService:   hs,
		payoutService: ps,
		firebaseAuth:  fa,
		adminEmail:    adminEmail,
	}
}

func (c *AdminController) RegisterRoutes(r chi.Router) {
	r.Route("/admin/hosts", func(r chi.Router) {
		// All routes in this group require admin authentication
		r.Use(auth.IsAdmin(c.firebaseAuth, c.adminEmail))

		r.Get("/applications", c.ListPendingApplications)
		r.Post("/{hostID}/approve", c.ApproveApplication)
		r.Post("/{hostID}/reject", c.RejectApplication)
	})

	r.Route("/admin/platform", func(r chi.Router) {
		// All routes in this group require admin authentication
		r.Use(auth.IsAdmin(c.firebaseAuth, c.adminEmail))

		r.Get("/balance", c.GetPlatformBalance)
		r.Post("/payout-methods", c.AddAdminPayoutMethod)
		r.Get("/payout-methods", c.ListAdminPayoutMethods)
		r.Put("/payout-methods/{methodID}/primary", c.SetAdminPrimaryMethod)
		r.Delete("/payout-methods/{methodID}", c.DeleteAdminPayoutMethod)
		r.Post("/withdraw", c.RequestAdminWithdrawal)
	})
}

// ── Request types ───────────────────────────────────────────────────────────

type RejectApplicationRequestBody struct {
	Reason string `json:"reason"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (c *AdminController) ListPendingApplications(w http.ResponseWriter, r *http.Request) {
	hosts, err := c.hostService.ListPendingApplications(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(w, http.StatusOK, hosts)
}

func (c *AdminController) ApproveApplication(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := c.hostService.ApproveApplication(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}

func (c *AdminController) RejectApplication(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	var req RejectApplicationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	host, err := c.hostService.RejectApplication(r.Context(), hostID, req.Reason)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}

// ── Admin Platform Payout Handlers ──────────────────────────────────────────

func (c *AdminController) GetPlatformBalance(w http.ResponseWriter, r *http.Request) {
	balance, err := c.payoutService.GetPlatformBalance(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(w, http.StatusOK, balance)
}

func (c *AdminController) AddAdminPayoutMethod(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		Type            string  `json:"type"`
		BankName        *string `json:"bank_name"`
		AccountType     *string `json:"account_type"`
		AccountNumber   *string `json:"account_number"`
		IFSC            *string `json:"ifsc"`
		BeneficiaryName *string `json:"beneficiary_name"`
		UPIID           *string `json:"upi_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	methodType := ""
	switch reqBody.Type {
	case "bank":
		methodType = "bank"
	case "upi":
		methodType = "upi"
	default:
		RespondError(w, http.StatusBadRequest, "Invalid payout method type")
		return
	}

	req := service.AddPayoutMethodRequest{
		Type:            models.PayoutMethodType(methodType),
		BankName:        reqBody.BankName,
		AccountType:     reqBody.AccountType,
		AccountNumber:   reqBody.AccountNumber,
		IFSC:            reqBody.IFSC,
		BeneficiaryName: reqBody.BeneficiaryName,
		UPIID:           reqBody.UPIID,
	}

	method, err := c.payoutService.AddAdminPayoutMethod(r.Context(), req)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, method)
}

func (c *AdminController) ListAdminPayoutMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := c.payoutService.ListAdminPayoutMethods(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(w, http.StatusOK, methods)
}

func (c *AdminController) SetAdminPrimaryMethod(w http.ResponseWriter, r *http.Request) {
	methodIDStr := chi.URLParam(r, "methodID")
	methodID, err := uuid.Parse(methodIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid method ID")
		return
	}

	if err := c.payoutService.SetAdminPrimaryMethod(r.Context(), methodID); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "Primary method updated"})
}

func (c *AdminController) DeleteAdminPayoutMethod(w http.ResponseWriter, r *http.Request) {
	methodIDStr := chi.URLParam(r, "methodID")
	methodID, err := uuid.Parse(methodIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid method ID")
		return
	}

	if err := c.payoutService.DeleteAdminPayoutMethod(r.Context(), methodID); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "Method deleted"})
}

func (c *AdminController) RequestAdminWithdrawal(w http.ResponseWriter, r *http.Request) {
	var reqBody struct {
		AmountCents    int64   `json:"amount_cents"`
		PayoutMethodID *string `json:"payout_method_id"`
		IdempotencyKey string  `json:"idempotency_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var methodID *uuid.UUID
	if reqBody.PayoutMethodID != nil {
		parsed, err := uuid.Parse(*reqBody.PayoutMethodID)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid payout method ID")
			return
		}
		methodID = &parsed
	}

	req := service.WithdrawalRequest{
		AmountCents:    reqBody.AmountCents,
		PayoutMethodID: methodID,
		IdempotencyKey: reqBody.IdempotencyKey,
	}

	payment, err := c.payoutService.RequestAdminWithdrawal(r.Context(), req)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, payment)
}
