package controller

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"myslotmate-backend/internal/lib/payment"
	"myslotmate-backend/internal/lib/payout"
	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// WebhookController handles callbacks from external payment providers.
type WebhookController struct {
	payoutService   service.PayoutService
	userService     service.UserService
	payoutProvider  payout.Provider
	paymentProvider payment.Provider
}

func NewWebhookController(ps service.PayoutService, us service.UserService, payoutProv payout.Provider, paymentProv payment.Provider) *WebhookController {
	return &WebhookController{
		payoutService:   ps,
		userService:     us,
		payoutProvider:  payoutProv,
		paymentProvider: paymentProv,
	}
}

func (c *WebhookController) RegisterRoutes(r chi.Router) {
	r.Route("/webhooks", func(r chi.Router) {
		r.Post("/payout", c.HandlePayoutWebhook)
		r.Post("/payment", c.HandlePaymentWebhook)
	})
}

// ── Razorpay webhook structures ─────────────────────────────────────────────

// RazorpayWebhookEvent is the top-level webhook payload from Razorpay.
// Docs: https://razorpay.com/docs/webhooks/payloads/payouts
type RazorpayWebhookEvent struct {
	Entity    string                 `json:"entity"`     // "event"
	AccountID string                 `json:"account_id"` // Razorpay account ID
	Event     string                 `json:"event"`      // e.g. "payout.processed", "payout.failed", "payout.reversed"
	Contains  []string               `json:"contains"`   // ["payout"]
	Payload   RazorpayWebhookPayload `json:"payload"`
	CreatedAt int64                  `json:"created_at"`
}

type RazorpayWebhookPayload struct {
	Payout struct {
		Entity RazorpayPayoutEntity `json:"entity"`
	} `json:"payout"`
}

type RazorpayPayoutEntity struct {
	ID            string `json:"id"`     // pout_xxxxx
	Entity        string `json:"entity"` // "payout"
	FundAccountID string `json:"fund_account_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"` // processed, reversed, failed, cancelled
	Mode          string `json:"mode"`
	Purpose       string `json:"purpose"`
	ReferenceID   string `json:"reference_id"` // our idempotency key / payment ID
	Narration     string `json:"narration"`
	FailureReason string `json:"failure_reason"`
	StatusDetails *struct {
		Description string `json:"description"`
		Source      string `json:"source"`
		Reason      string `json:"reason"`
	} `json:"status_details,omitempty"`
	Error *struct {
		Description string `json:"description"`
		Source      string `json:"source"`
		Reason      string `json:"reason"`
	} `json:"error,omitempty"`
}

// mapRazorpayWebhookEvent converts a Razorpay event name to our internal status.
func mapRazorpayWebhookEvent(event string) string {
	switch event {
	case "payout.processed":
		return "completed"
	case "payout.failed":
		return "failed"
	case "payout.reversed":
		return "reversed"
	case "payout.queued", "payout.initiated":
		return "processing"
	default:
		return ""
	}
}

func (c *WebhookController) HandlePayoutWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// 2. Verify Razorpay webhook signature (X-Razorpay-Signature header)
	signature := r.Header.Get("X-Razorpay-Signature")
	if !c.payoutProvider.ValidateWebhookSignature(body, signature) {
		RespondError(w, http.StatusUnauthorized, "Invalid webhook signature")
		return
	}

	// 3. Parse Razorpay webhook event
	var event RazorpayWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	// 4. Map event to internal status
	status := mapRazorpayWebhookEvent(event.Event)
	if status == "" {
		// Unrecognised event — acknowledge so Razorpay doesn't retry
		log.Printf("[webhook] ignoring unhandled Razorpay event: %s", event.Event)
		RespondSuccess(w, http.StatusOK, map[string]string{"message": "event ignored"})
		return
	}

	// 5. Extract our payment ID from the reference_id field.
	//    We set reference_id = idempotency key when creating the payout;
	//    the idempotency key is our payment UUID.
	payoutEntity := event.Payload.Payout.Entity
	paymentID, err := uuid.Parse(payoutEntity.ReferenceID)
	if err != nil {
		log.Printf("[webhook] could not parse reference_id as UUID: %s", payoutEntity.ReferenceID)
		RespondError(w, http.StatusBadRequest, "Invalid reference_id in payout entity")
		return
	}

	// 6. Build error message from available fields
	var errMsg string
	if payoutEntity.FailureReason != "" {
		errMsg = payoutEntity.FailureReason
	} else if payoutEntity.StatusDetails != nil {
		errMsg = payoutEntity.StatusDetails.Description
	} else if payoutEntity.Error != nil {
		errMsg = payoutEntity.Error.Description
	}

	// 7. Delegate to service layer
	if err := c.payoutService.HandlePayoutWebhook(r.Context(), paymentID, status, errMsg); err != nil {
		log.Printf("[webhook] error processing payout webhook for payment %s: %v", paymentID, err)
		RespondError(w, http.StatusInternalServerError, "Failed to process webhook")
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "webhook processed"})
}

// ── Payment (collection) webhook ────────────────────────────────────────────

// RazorpayPaymentWebhookEvent is the top-level payload for payment.captured / payment.failed events.
type RazorpayPaymentWebhookEvent struct {
	Entity    string `json:"entity"` // "event"
	AccountID string `json:"account_id"`
	Event     string `json:"event"` // "payment.captured", "payment.failed"
	Payload   struct {
		Payment struct {
			Entity struct {
				ID      string `json:"id"`       // pay_xxxxx
				OrderID string `json:"order_id"` // order_xxxxx
				Amount  int64  `json:"amount"`
				Status  string `json:"status"` // "captured", "failed"
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
	CreatedAt int64 `json:"created_at"`
}

func (c *WebhookController) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	signature := r.Header.Get("X-Razorpay-Signature")
	if !c.paymentProvider.ValidateWebhookSignature(body, signature) {
		RespondError(w, http.StatusUnauthorized, "Invalid webhook signature")
		return
	}

	var event RazorpayPaymentWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	// We only act on payment.captured — the money is with us.
	if event.Event != "payment.captured" {
		log.Printf("[webhook/payment] ignoring event: %s", event.Event)
		RespondSuccess(w, http.StatusOK, map[string]string{"message": "event ignored"})
		return
	}

	orderID := event.Payload.Payment.Entity.OrderID
	paymentID := event.Payload.Payment.Entity.ID
	if orderID == "" || paymentID == "" {
		RespondError(w, http.StatusBadRequest, "Missing order_id or payment id in webhook payload")
		return
	}

	if err := c.userService.CreditWalletFromWebhook(r.Context(), orderID, paymentID); err != nil {
		log.Printf("[webhook/payment] error crediting wallet for order %s: %v", orderID, err)
		RespondError(w, http.StatusInternalServerError, "Failed to process payment webhook")
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "payment webhook processed"})
}
