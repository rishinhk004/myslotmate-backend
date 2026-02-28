package models

// AccountOwnerType is the owner of an account (user or host).
type AccountOwnerType string

const (
	AccountOwnerUser AccountOwnerType = "user"
	AccountOwnerHost AccountOwnerType = "host"
)

// BookingStatus represents the lifecycle of a booking.
type BookingStatus string

const (
	BookingStatusPending   BookingStatus = "pending"
	BookingStatusConfirmed BookingStatus = "confirmed"
	BookingStatusCancelled BookingStatus = "cancelled"
	BookingStatusRefunded  BookingStatus = "refunded"
)

// PaymentType is the kind of payment transaction.
type PaymentType string

const (
	PaymentTypeBooking    PaymentType = "booking"
	PaymentTypeWithdrawal PaymentType = "withdrawal"
	PaymentTypeRefund     PaymentType = "refund"
	PaymentTypePayout     PaymentType = "payout"
	PaymentTypeTopup      PaymentType = "topup"
)

// PaymentStatus is the status of a payment.
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusReversed   PaymentStatus = "reversed"
)

// PayoutMethodType is the type of payout method (bank or UPI).
type PayoutMethodType string

const (
	PayoutMethodBank PayoutMethodType = "bank"
	PayoutMethodUPI  PayoutMethodType = "upi"
)

// SupportTicketStatus is the status of a support ticket.
type SupportTicketStatus string

const (
	SupportTicketOpen        SupportTicketStatus = "open"
	SupportTicketInProgress  SupportTicketStatus = "in_progress"
	SupportTicketResolved    SupportTicketStatus = "resolved"
	SupportTicketClosed      SupportTicketStatus = "closed"
)

// FraudFlagType is the reason for a fraud flag.
type FraudFlagType string

const (
	FraudFlagAbnormalBookingSpike FraudFlagType = "abnormal_booking_spike"
	FraudFlagPaymentAbuse         FraudFlagType = "payment_abuse"
	FraudFlagSuspiciousActivity   FraudFlagType = "suspicious_activity"
	FraudFlagManualBlock          FraudFlagType = "manual_block"
)
