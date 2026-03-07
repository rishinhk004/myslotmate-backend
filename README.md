# MySlotMate — Backend

**A production-grade event booking platform backend built with Go, following Clean Architecture and enterprise design patterns.**

MySlotMate allows users to discover and book event slots, and enables verified hosts to create and manage events, track earnings, and communicate with attendees — all backed by Aadhar-based identity verification, real-time WebSocket updates, and a wallet-based payment/payout system powered by Razorpay.

---

## Table of Contents

- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Design Patterns](#design-patterns)
- [Project Structure](#project-structure)
- [Database Schema](#database-schema)
- [Payment & Wallet Flow](#payment--wallet-flow)
- [Booking Flow](#booking-flow)
- [Aadhar Verification Flow](#aadhar-verification-flow)
- [Payout Flow](#payout-flow)
- [Fraud Prevention](#fraud-prevention)
- [API Endpoints](#api-endpoints)
- [Getting Started](#getting-started)
- [Configuration](#configuration)
- [Migrations](#migrations)

---

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | **Go 1.24** |
| HTTP Router | [Chi v5](https://github.com/go-chi/chi) |
| Database | **PostgreSQL** (via [pgx v5](https://github.com/jackc/pgx)) |
| Auth | **Firebase Admin SDK** (ID token verification) |
| Real-time | **Socket.IO** ([go-socket.io](https://github.com/googollee/go-socket.io)) |
| Identity (KYC) | **Setu OKYC** (Aadhar verification) |
| Payouts | **Razorpay Payouts API** (RazorpayX — bank & UPI transfers) |
| Config | [godotenv](https://github.com/joho/godotenv) (`.env` file) |
| UUID | [google/uuid](https://github.com/google/uuid) |

---

## Architecture

The project follows **Clean Architecture** principles with strict layer separation:

```
┌──────────────────────────────────────────────────────────┐
│                    Transport Layer                        │
│         Router (Chi) → Controllers (HTTP Handlers)       │
├──────────────────────────────────────────────────────────┤
│                  Business Logic Layer                     │
│   Services (Rules, Orchestration, Fee Calculation, etc.) │
├──────────────────────────────────────────────────────────┤
│                   Data Access Layer                       │
│             Repositories (SQL via pgx)                   │
├──────────────────────────────────────────────────────────┤
│                    Infrastructure                         │
│  Worker Pool │ Event Dispatcher │ Socket.IO │ Identity    │
│              │  Payout Provider (Razorpay)                │
└──────────────────────────────────────────────────────────┘
                          │
                    ┌─────┴─────┐
                    │ PostgreSQL │
                    └───────────┘
```

### Layer Responsibilities

| Layer | Package | Responsibility |
|-------|---------|---------------|
| **Transport** | `internal/server`, `internal/controller` | HTTP routing, middleware, request decoding, JSON response formatting. **No business logic.** |
| **Business Logic** | `internal/service` | Core rules (overbooking prevention, fee split, wallet debit/credit, verification flow), orchestrates repos + infra. |
| **Data Access** | `internal/repository` | Direct SQL operations. Abstraction over PostgreSQL; mockable for unit tests. |
| **Infrastructure** | `internal/lib/*` | Reusable components — Event Bus, Worker Pool, Identity (KYC), Payout (Razorpay), Real-time (Socket.IO). |

### Dependency Wiring (Composition Root)

All dependencies are wired in `cmd/api/run.go`:

```
main() → Config → DB → Dispatcher → WorkerPool → Firebase
       → Repositories → Identity Provider → Payout Provider (Razorpay)
       → Services → Controllers → Router → HTTP Server
```

---

## Design Patterns

### 1. Singleton
**Where:** `EventDispatcher`, Database Connection  
**Why:** Single instance of the event bus and connection pool across the entire application lifecycle.  
**Implementation:** `sync.Once` in `event.GetDispatcher()`.

### 2. Observer (Pub/Sub)
**Where:** `internal/lib/event/dispatcher.go`  
**Why:** Decouples services. When a booking is created, `BookingService` publishes `BookingCreated` — other subsystems (email, analytics, notifications) subscribe independently without touching booking code.

```go
type Observer interface {
    OnNotify(event EventName, data interface{})
}

// Events: UserCreated, HostCreated, EventCreated, BookingCreated, BookingConfirmed,
//         BookingCancelled, PayoutCompleted, PayoutFailed, PaymentCreated
dispatcher.Subscribe("booking_created", analyticsObserver)
dispatcher.Publish("booking_created", bookingData)
```

### 3. Strategy
**Where:** `internal/lib/identity/` (Aadhar KYC), `internal/lib/payout/` (Razorpay payouts)  
**Why:** Swap providers without changing service code. The service depends on the interface, not the implementation.

```go
// Identity Strategy
type AadharProvider interface {
    InitiateVerification(ctx, aadharNumber) (transactionID, error)
    VerifyOTP(ctx, transactionID, otp) (*AadharVerificationResult, error)
}
// Implementation: SetuAadharProvider (Setu OKYC API)

// Payout Strategy
type Provider interface {
    InitiateTransfer(ctx, TransferRequest) (*TransferResponse, error)
    CheckStatus(ctx, providerRefID) (*TransferResponse, error)
    ValidateWebhookSignature(payload, signature) bool
}
// Implementation: RazorpayProvider (RazorpayX Payouts API)
```

### 4. Repository
**Where:** `internal/repository/*.go`  
**Why:** Abstracts database access behind interfaces, enabling service-layer unit tests with mocked repositories.

Repositories: `UserRepository`, `HostRepository`, `EventRepository`, `BookingRepository`, `ReviewRepository`, `InboxRepository`, `AccountRepository`, `PaymentRepository`, `PayoutRepository`.

### 5. Executor / Worker Pool
**Where:** `internal/lib/worker/pool.go`  
**Why:** Offloads heavy operations (emails, image processing) to background goroutines without blocking HTTP handlers.

```go
pool := worker.NewWorkerPool(5, 100)  // 5 workers, 100-item queue
pool.Start()
pool.Submit(func() { sendWelcomeEmail(user) })
```

Falls back to a standalone goroutine if the queue is full. Supports graceful shutdown with context timeout.

### 6. Factory / Dependency Injection
**Where:** All `New*()` constructors  
**Why:** Encapsulates creation logic; wires dependencies at the composition root (`cmd/api/run.go`).

---

## Project Structure

```
myslotmate-backend/
├── cmd/
│   ├── api/run.go                  # Application entry point & DI wiring
│   ├── checkdb/run.go              # DB connectivity check utility
│   └── migrate/run.go              # Migration runner
├── config/
│   └── firebase-service-account.json
├── internal/
│   ├── auth/
│   │   └── handler.go              # Firebase ID token verification
│   ├── config/
│   │   └── config.go               # Env-based configuration loader
│   ├── controller/                 # HTTP handlers (transport layer)
│   │   ├── booking_controller.go   # Create/confirm/cancel bookings
│   │   ├── event_controller.go     # Create/list events
│   │   ├── host_controller.go      # Create/get host profiles
│   │   ├── inbox_controller.go     # Broadcast messages to attendees
│   │   ├── payout_controller.go    # Payout methods, withdrawals, earnings
│   │   ├── response.go            # Standardized JSON response helpers
│   │   ├── review_controller.go    # Submit/list reviews
│   │   ├── user_controller.go      # Signup, Aadhar verification
│   │   └── webhook_controller.go   # Razorpay payout webhooks
│   ├── db/
│   │   └── db.go                   # PostgreSQL connection (pgx)
│   ├── firebase/
│   │   └── firebase.go             # Firebase Admin SDK initialization
│   ├── lib/
│   │   ├── event/
│   │   │   └── dispatcher.go       # Singleton Observer (event bus)
│   │   ├── identity/
│   │   │   ├── aadhar_provider.go  # Strategy interface (KYC)
│   │   │   └── setu_provider.go    # Setu OKYC implementation
│   │   ├── payout/
│   │   │   ├── provider.go         # Strategy interface (payouts)
│   │   │   └── razorpay_provider.go# Razorpay Payouts API implementation
│   │   ├── realtime/
│   │   │   └── socket_service.go   # Socket.IO server
│   │   └── worker/
│   │       └── pool.go             # Background worker pool (executor)
│   ├── models/                     # Domain structs & enums
│   │   ├── account.go              # Wallet account
│   │   ├── booking.go              # Booking with fee breakdown
│   │   ├── enums.go                # All enum types
│   │   ├── event.go                # Event
│   │   ├── fraud.go                # Fraud flags
│   │   ├── host_earnings.go        # Aggregate earnings
│   │   ├── host.go                 # Host profile
│   │   ├── inbox.go                # Inbox messages
│   │   ├── payment.go              # Transaction ledger
│   │   ├── payout_method.go        # Bank/UPI payout methods
│   │   ├── platform_settings.go    # Fee config
│   │   ├── review.go               # Reviews with AI sentiment
│   │   ├── support.go              # Support tickets
│   │   └── user.go                 # User
│   ├── repository/                 # Data access layer (SQL)
│   │   ├── account_repository.go   # Wallet CRUD, credit/debit
│   │   ├── booking_repository.go   # Booking CRUD, status updates
│   │   ├── errors.go              # Sentinel errors (ErrInsufficientBalance, etc.)
│   │   ├── event_repository.go     # Event CRUD
│   │   ├── host_repository.go      # Host CRUD
│   │   ├── inbox_repository.go     # Inbox messages
│   │   ├── payment_repository.go   # Payment/transaction ledger
│   │   ├── payout_repository.go    # Payout methods, host earnings, platform settings
│   │   ├── review_repository.go    # Review CRUD
│   │   └── user_repository.go      # User CRUD
│   ├── server/
│   │   └── router.go               # Chi router, middleware, route mounting
│   └── service/                    # Business logic layer
│       ├── booking_service.go      # Booking with wallet debit/credit, fee split
│       ├── event_service.go        # Event management
│       ├── host_service.go         # Host profile management
│       ├── inbox_service.go        # Broadcast messaging
│       ├── payout_service.go       # Withdrawal, earnings, webhook handling
│       ├── review_service.go       # Review management
│       └── user_service.go         # Signup, Aadhar verification
└── migrations/                     # PostgreSQL migration files
    ├── 20260228120000_init_schema.sql
    ├── 20260228130000_add_processing_status.sql
    └── 20260228130001_earnings_payouts_schema.sql
```

---

## Database Schema

### Entity Relationship Diagram

```
┌──────────┐     1:1      ┌──────────┐      1:N      ┌──────────┐
│   User   │─────────────▶│   Host   │──────────────▶│  Event   │
│          │  (verified)   │          │   (host_id)   │          │
└────┬─────┘               └────┬─────┘               └────┬─────┘
     │                          │                          │
     │ 1:1                      │ 1:1                      │ 1:N
     ▼                          ▼                          ▼
┌──────────┐            ┌────────────┐             ┌──────────┐
│ Account  │            │HostEarnings│             │ Booking  │◀── User (N:1)
│ (wallet) │            └────────────┘             └────┬─────┘
└──────────┘                    │                       │
                                │ 1:N                   │ 1:1
                                ▼                       ▼
                        ┌──────────────┐          ┌──────────┐
                        │ PayoutMethod │          │ Payment  │
                        └──────────────┘          └──────────┘

  Event ◀──1:N── Review ──N:1──▶ User
  Event ◀──1:N── InboxMessage ──N:1──▶ Host
  User  ◀──1:N── SupportTicket
  User  ◀──1:N── FraudFlag
```

### Tables

#### `users`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK, default `uuid_generate_v4()` |
| `auth_uid` | VARCHAR | UNIQUE, NOT NULL |
| `name` | VARCHAR | NOT NULL |
| `phn_number` | VARCHAR | |
| `email` | VARCHAR | NOT NULL |
| `account_id` | UUID | FK → `accounts` |
| `is_verified` | BOOLEAN | DEFAULT `false` |
| `verified_at` | TIMESTAMPTZ | |
| `created_at` | TIMESTAMPTZ | DEFAULT `now()` |
| `updated_at` | TIMESTAMPTZ | DEFAULT `now()` |

#### `hosts`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `user_id` | UUID | UNIQUE, FK → `users` |
| `name` | VARCHAR | NOT NULL |
| `phn_number` | VARCHAR | |
| `account_id` | UUID | FK → `accounts` |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

> **Trigger:** `trg_host_user_must_be_verified` — prevents host creation unless `users.is_verified = true`.

#### `accounts` (Wallet)
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `owner_type` | ENUM | `user` \| `host` |
| `owner_id` | UUID | UNIQUE per `(owner_type, owner_id)` |
| `balance_cents` | BIGINT | CHECK `≥ 0` |
| `bank_details` | JSONB | |

> **Auto-created** via triggers on `users` and `hosts` insert. Wallet can never go negative.

#### `events`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `host_id` | UUID | FK → `hosts` |
| `name` | VARCHAR | NOT NULL |
| `time` | TIMESTAMPTZ | NOT NULL |
| `end_time` | TIMESTAMPTZ | |
| `capacity` | INT | CHECK `≥ 0` |
| `ai_suggestion` | TEXT | |

#### `bookings`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `event_id` | UUID | FK → `events` |
| `user_id` | UUID | FK → `users` |
| `quantity` | INT | CHECK `> 0` |
| `status` | ENUM | `pending` \| `confirmed` \| `cancelled` \| `refunded` |
| `payment_id` | UUID | FK → `payments` |
| `idempotency_key` | VARCHAR | UNIQUE |
| `amount_cents` | BIGINT | Total booking value |
| `service_fee_cents` | BIGINT | Platform fee (15%) |
| `net_earning_cents` | BIGINT | Host net (85%) |
| `cancelled_at` | TIMESTAMPTZ | Set when status → cancelled |

> **Overbooking prevention:** Service layer checks `SUM(quantity) WHERE status IN ('pending','confirmed') < event.capacity` before confirming.

#### `payments` (Transaction Ledger)
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `idempotency_key` | VARCHAR | UNIQUE |
| `account_id` | UUID | FK → `accounts` |
| `type` | ENUM | `booking` \| `withdrawal` \| `refund` \| `payout` \| `topup` |
| `reference_id` | UUID | e.g. Booking ID |
| `amount_cents` | BIGINT | |
| `status` | ENUM | `pending` \| `processing` \| `completed` \| `failed` \| `reversed` |
| `payout_method_id` | UUID | FK → `payout_methods` |
| `display_reference` | VARCHAR | Human-readable (e.g. `TXN-88234`) |
| `retry_count` | INT | DEFAULT `0` |
| `last_error` | TEXT | |

#### `payout_methods`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `host_id` | UUID | FK → `hosts` |
| `type` | ENUM | `bank` \| `upi` |
| `bank_name` | VARCHAR | Required for type=bank |
| `account_type` | VARCHAR | `checking` \| `savings` |
| `last_four_digits` | VARCHAR | Masked display |
| `account_number_encrypted` | TEXT | Encrypted (never exposed in JSON) |
| `ifsc` | VARCHAR | Bank IFSC code |
| `beneficiary_name` | VARCHAR | Account holder name |
| `upi_id` | VARCHAR | Required for type=upi (e.g. `user@okhdfc`) |
| `is_verified` | BOOLEAN | |
| `is_primary` | BOOLEAN | |

#### `host_earnings`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `host_id` | UUID | UNIQUE, FK → `hosts` |
| `total_earnings_cents` | BIGINT | Lifetime earnings |
| `pending_clearance_cents` | BIGINT | Funds awaiting clearance |
| `estimated_clearance_at` | TIMESTAMPTZ | |

> **Auto-created** via trigger on `hosts` insert.

#### `platform_settings`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `key` | VARCHAR | UNIQUE |
| `value` | JSONB | |

> Seeded: `platform_fee → { host_percentage: 85, platform_percentage: 15 }`

#### `reviews`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `event_id` | UUID | FK → `events` |
| `user_id` | UUID | FK → `users` |
| `name` | VARCHAR | |
| `description` | TEXT | NOT NULL |
| `reply` | TEXT[] | |
| `sentiment_score` | FLOAT | AI-generated |

#### `inbox_messages`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `event_id` | UUID | FK → `events` |
| `host_id` | UUID | FK → `hosts` |
| `message` | TEXT | NOT NULL |
| `created_at` | TIMESTAMPTZ | |

#### `support_tickets`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `user_id` | UUID | FK → `users` |
| `subject` | VARCHAR | |
| `messages` | JSONB | `[{ sender, text, created_at }]` |
| `status` | ENUM | `open` \| `in_progress` \| `resolved` \| `closed` |

#### `fraud_flags`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `user_id` | UUID | FK → `users` |
| `type` | ENUM | `abnormal_booking_spike` \| `payment_abuse` \| `suspicious_activity` \| `manual_block` |
| `reason` | TEXT | |
| `blocked_at` | TIMESTAMPTZ | |
| `blocked_until` | TIMESTAMPTZ | Nullable — null = indefinite |
| `is_active` | BOOLEAN | |

### Enums

| Enum Type | Values |
|-----------|--------|
| `account_owner_type` | `user`, `host` |
| `booking_status` | `pending`, `confirmed`, `cancelled`, `refunded` |
| `payment_type` | `booking`, `withdrawal`, `refund`, `payout`, `topup` |
| `payment_status` | `pending`, `processing`, `completed`, `failed`, `reversed` |
| `payout_method_type` | `bank`, `upi` |
| `support_ticket_status` | `open`, `in_progress`, `resolved`, `closed` |
| `fraud_flag_type` | `abnormal_booking_spike`, `payment_abuse`, `suspicious_activity`, `manual_block` |

### Database Triggers

| Trigger | Purpose |
|---------|---------|
| `trg_host_user_must_be_verified` | Prevents host creation if `user.is_verified` is `false` |
| `set_updated_at()` | Auto-updates `updated_at` on row modification (all tables) |
| `create_user_account()` | Auto-creates a wallet `Account` when a `User` is inserted |
| `create_host_account()` | Auto-creates a wallet `Account` when a `Host` is inserted |
| `create_host_earnings()` | Auto-creates a `HostEarnings` row when a `Host` is inserted |

---

## Payment & Wallet Flow

MySlotMate uses an **internal wallet** model (similar to Paytm, Ola, etc.). Users and hosts each have a wallet (`Account`) with a `balance_cents` field. All money movement happens **wallet-to-wallet**, with external transfers only at the edges (topup and payout).

```
External  ──topup──►  User Wallet  ──booking──►  Host Wallet  ──payout──►  Bank/UPI
                      (Account)                   (Account)                (Razorpay)
```

### Core Financial Concepts

| Entity | Purpose |
|--------|---------|
| **Account** | Wallet per user/host. `balance_cents` is the source of truth for available funds. Auto-created via DB trigger. |
| **Payment** | Transaction ledger. Every wallet movement (topup, booking, refund, payout) is recorded as a Payment with idempotency. |
| **Booking** | Stores fee breakdown: `amount_cents`, `service_fee_cents` (15%), `net_earning_cents` (85%). |
| **HostEarnings** | Aggregate view: `total_earnings_cents`, `pending_clearance_cents`. Auto-created via DB trigger. |
| **PayoutMethod** | Host's bank account or UPI ID for withdrawals. Supports multiple methods with one primary. |
| **PlatformSettings** | Configurable fee split (default 85% host / 15% platform). |

### Payment Types

| Type | Direction | Description |
|------|-----------|-------------|
| `topup` | External → User wallet | User adds money to wallet |
| `booking` | User wallet → Host wallet | User pays for event tickets |
| `refund` | Host wallet → User wallet | Reversed booking payment |
| `payout` | Host wallet → Bank/UPI | Host withdraws via Razorpay |
| `withdrawal` | Host wallet → External | Host cashes out |

### Payment Status Lifecycle

```
pending ──► processing ──► completed
                       └──► failed (retry_count++)
completed ──► reversed (for refunds)
```

### Safety Features

- **Idempotency:** `idempotency_key` (unique) prevents duplicate charges from retried requests
- **Non-negative balance:** `CHECK (balance_cents >= 0)` at the database level — wallet can never go negative
- **Overdraft protection:** Debit query uses `WHERE balance_cents >= amount` so the update silently fails if insufficient
- **Retry tracking:** `retry_count` + `last_error` for failed payment recovery
- **Fraud checks:** Active fraud flags block booking and payment operations

---

## Booking Flow

### Creating a Booking

```
1.  User sends POST /bookings with event_id, quantity, and idempotency_key
2.  Service checks for active fraud flags on the user
3.  Service checks idempotency — returns existing booking if key already used
4.  Service checks event capacity:
      SUM(quantity) WHERE status IN ('pending','confirmed') + new_quantity ≤ capacity
5.  Fee calculated from PlatformSettings (default 85/15):
      total    = price × quantity
      fee      = total × 15%
      net      = total − fee    (85%)
6.  User wallet debited: Account.balance_cents -= total
7.  Payment record created: type=booking, reference_id=booking.id, status=completed
8.  Host wallet credited: Account.balance_cents += net
9.  HostEarnings.total_earnings_cents += net
10. Booking created with status=confirmed
11. BookingCreated event published (Observer pattern)
```

### Confirming a Booking

```
1.  POST /bookings/{bookingID}/confirm
2.  Booking status → confirmed
3.  BookingConfirmed event published
```

### Cancelling a Booking (Full Refund)

```
1.  POST /bookings/{bookingID}/cancel
2.  Booking.status → cancelled, cancelled_at = now
3.  User wallet credited: Account.balance_cents += amount_cents (full refund)
4.  Host wallet debited: Account.balance_cents -= net_earning_cents
5.  Refund payment record created: type=refund, reference_id=booking.id
6.  HostEarnings.pending_clearance_cents adjusted
7.  BookingCancelled event published
```

---

## Aadhar Verification Flow

This flow demonstrates the interaction between multiple layers and design patterns (Strategy, Observer, Worker Pool, Repository).

```
┌────────┐          ┌────────────┐         ┌─────────────┐        ┌──────────────┐
│ Client │          │ Controller │         │   Service   │        │ SetuProvider │
└───┬────┘          └─────┬──────┘         └──────┬──────┘        └──────┬───────┘
    │                     │                       │                      │
    │ POST /auth/verify-  │                       │                      │
    │  aadhar/init        │                       │                      │
    │────────────────────►│                       │                      │
    │                     │  UserService.         │                      │
    │                     │  InitiateVerification │                      │
    │                     │──────────────────────►│                      │
    │                     │                       │  Check if already    │
    │                     │                       │  verified (UserRepo) │
    │                     │                       │──────┐               │
    │                     │                       │◄─────┘               │
    │                     │                       │                      │
    │                     │                       │  AadharProvider.     │
    │                     │                       │  InitiateVerification│
    │                     │                       │─────────────────────►│
    │                     │                       │                      │ Call Setu
    │                     │                       │                      │ OKYC API
    │                     │                       │◄─────────────────────│
    │                     │◄──────────────────────│                      │
    │◄────────────────────│  { transaction_id }   │                      │
    │                     │                       │                      │
    │ POST /auth/verify-  │                       │                      │
    │  aadhar/complete    │                       │                      │
    │  (transaction_id    │                       │                      │
    │   + OTP)            │                       │                      │
    │────────────────────►│                       │                      │
    │                     │  VerifyOTP            │                      │
    │                     │──────────────────────►│                      │
    │                     │                       │  AadharProvider.     │
    │                     │                       │  VerifyOTP           │
    │                     │                       │─────────────────────►│
    │                     │                       │◄─────────────────────│
    │                     │                       │                      │
    │                     │                       │  UserRepo.           │
    │                     │                       │  SetVerified(true)   │
    │                     │                       │──────┐               │
    │                     │                       │◄─────┘               │
    │                     │                       │                      │
    │                     │                       │  Publish             │
    │                     │                       │  UserVerified event  │
    │                     │                       │──────┐               │
    │                     │                       │◄─────┘               │
    │                     │◄──────────────────────│                      │
    │◄────────────────────│  "User verified"      │                      │
```

**Step-by-step:**

1. Client POSTs to `/auth/verify-aadhar/init` with Aadhar number
2. `UserController` receives request, calls `UserService`
3. `UserService` checks DB (via `UserRepository`) to ensure user isn't already verified
4. `UserService` calls `AadharProvider.InitiateVerification` (Strategy pattern — uses `SetuAadharProvider` in production)
5. Setu OKYC API sends OTP to user's Aadhar-linked mobile
6. Transaction ID returned to client
7. Client POSTs OTP to `/auth/verify-aadhar/complete`
8. `UserService` validates OTP via `AadharProvider.VerifyOTP`
9. `UserRepository.SetVerified(true)` marks user as verified
10. `UserVerified` event published (Observer pattern)
11. Background worker sends "Verification Successful" notification (Worker Pool pattern)

> After verification, the user can create a Host profile. The `trg_host_user_must_be_verified` trigger enforces this at the DB level.

---

## Payout Flow

Hosts withdraw earnings to their bank account or UPI via Razorpay Payouts API (RazorpayX).

### Adding a Payout Method

```
1. Host sends POST /payouts/methods with bank details or UPI ID
2. Account number is masked (last 4 digits stored as display)
3. First method is automatically set as primary
```

### Requesting a Withdrawal

```
1.  Host sends POST /payouts/withdraw with host_id, amount, and payout_method_id
2.  Service checks for active fraud flags
3.  Service checks idempotency (prevents duplicate withdrawals)
4.  Service validates: amount ≤ Account.balance_cents
5.  Host wallet debited: Account.balance_cents -= amount
6.  Payment record created: type=payout, status=processing
7.  Razorpay Payouts API called (POST /v1/payouts) with:
    - Inline fund account creation (bank_account or vpa)
    - IMPS mode for bank, UPI mode for UPI
    - Reference ID = payment UUID (for idempotency)
8.  On Razorpay success → Payment status updated with provider ref ID
9.  On Razorpay failure → Wallet credited back, Payment status=failed
```

### Webhook Processing (Async Settlement)

```
1.  Razorpay sends POST /webhooks/payout with event payload
2.  Webhook controller verifies X-Razorpay-Signature (HMAC-SHA256)
3.  Event type mapped to internal status:
      payout.processed → completed
      payout.failed    → failed (wallet credited back)
      payout.reversed  → reversed (wallet credited back)
4.  Payment record updated with final status
5.  PayoutCompleted / PayoutFailed event published
```

### Razorpay Integration Details

| Feature | Implementation |
|---------|---------------|
| API | RazorpayX Payouts API (`POST /v1/payouts`) |
| Auth | HTTP Basic Auth (KeyID:KeySecret) |
| Fund Account | Inline creation (no pre-registration needed) |
| Bank Transfer | IMPS mode (instant, up to ₹5L) |
| UPI Transfer | UPI mode via VPA address |
| Webhook | HMAC-SHA256 signature verification |
| Idempotency | `reference_id` = payment UUID |

### Earnings Dashboard

Available via `GET /payouts/earnings/{hostID}`:

```
Available balance  = Account.balance_cents       (host's wallet)
Total earnings     = HostEarnings.total_earnings_cents
Pending clearance  = HostEarnings.pending_clearance_cents
Fee config         = PlatformSettings (85% host / 15% platform)
```

---

## Fraud Prevention

Guards the money flow by flagging/blocking suspicious users.

| Flag Type | Trigger |
|-----------|---------|
| `abnormal_booking_spike` | User books unusually high volume in short time |
| `payment_abuse` | Repeated failed payments, chargebacks |
| `suspicious_activity` | General suspicious behavior |
| `manual_block` | Admin manually blocks a user |

When `is_active = true`, the booking and payout services check for active fraud flags and reject operations. The `blocked_until` field supports temporary blocks (null = indefinite).

---

## API Endpoints

All responses follow a standardized JSON envelope:

```json
{
  "success": true,
  "data": { ... },
  "message": "...",
  "error": "..."
}
```

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Returns `"ok"` — liveness probe |

### Authentication & User

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/auth/signup` | Register a new user (Firebase UID + email) |
| `POST` | `/auth/verify-aadhar/init` | Initiate Aadhar OTP verification via Setu |
| `POST` | `/auth/verify-aadhar/complete` | Submit OTP to complete KYC verification |

<details>
<summary>Request / Response Examples</summary>

**POST `/auth/signup`**
```json
{
  "auth_uid": "firebase-uid-abc123",
  "email": "user@example.com",
  "name": "Jane Doe",
  "phn_number": "+919876543210"
}
// → 201 { "success": true, "data": { "id": "uuid", ... } }
// → 409 if user already exists
```

**POST `/auth/verify-aadhar/init`**
```json
{
  "user_id": "uuid",
  "aadhar_number": "123456789012"
}
// → 200 { "success": true, "data": { "transaction_id": "...", "message": "OTP sent" } }
```

**POST `/auth/verify-aadhar/complete`**
```json
{
  "user_id": "uuid",
  "transaction_id": "...",
  "otp": "123456"
}
// → 200 { "success": true, "data": { "message": "User verified successfully" } }
```
</details>

### Hosts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/hosts/` | Create a host profile (requires verified user) |
| `GET` | `/hosts/me` | Get own host profile |

### Events

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/events/` | Create a new event (host only) |
| `GET` | `/events/host/{hostID}` | List all events for a host |

### Bookings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/bookings/` | Book tickets (overbooking-safe, wallet debit, fee split) |
| `POST` | `/bookings/{bookingID}/confirm` | Confirm a pending booking |
| `POST` | `/bookings/{bookingID}/cancel` | Cancel booking (full refund to wallet) |
| `GET` | `/bookings/user/{userID}` | Get booking history for a user |

<details>
<summary>Request Example</summary>

**POST `/bookings/`**
```json
{
  "user_id": "uuid",
  "event_id": "uuid",
  "quantity": 2,
  "idempotency_key": "unique-request-key"
}
// → 201 (auto-calculates amount_cents, service_fee_cents, net_earning_cents)
// → Wallet debited, host credited, payment record created
```
</details>

### Payouts & Earnings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/payouts/methods` | Add a bank account or UPI payout method |
| `GET` | `/payouts/methods/{hostID}` | List payout methods for a host |
| `PUT` | `/payouts/methods/{methodID}/primary` | Set a payout method as primary |
| `DELETE` | `/payouts/methods/{methodID}` | Remove a payout method |
| `POST` | `/payouts/withdraw` | Request a withdrawal (via Razorpay) |
| `GET` | `/payouts/earnings/{hostID}` | Get earnings summary (balance, total, pending) |
| `GET` | `/payouts/history/{hostID}` | Get payout transaction history |

### Reviews

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/reviews/` | Submit a review for an event |
| `GET` | `/reviews/event/{eventID}` | List reviews for an event |

### Inbox (Broadcasts)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/inbox/broadcast` | Host broadcasts a message to event attendees |
| `GET` | `/inbox/host/{hostID}` | Get all broadcast messages for a host |

> Broadcasts are also pushed in real-time via **Socket.IO** to room `event_{eventID}`.

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/webhooks/payout` | Razorpay payout webhook (signature verified) |

### Real-time (Socket.IO)

| Endpoint | Description |
|----------|-------------|
| `/socket.io/*` | WebSocket endpoint |

**Client Events:**
- `join_room` — Join a room (e.g. `event_{eventID}`)

**Server Events:**
- `inbox_update` — Pushed when host broadcasts a message

---

## Getting Started

### Prerequisites

- **Go 1.24+**
- **PostgreSQL 15+**
- **Firebase project** with service account credentials
- **Razorpay account** with RazorpayX (Payouts) enabled
- **Setu account** for Aadhar OKYC

### Setup

```bash
# 1. Clone
git clone <repo-url>
cd myslotmate-backend

# 2. Configure environment
cp .env.example .env
# Edit .env with your database URL, Firebase config, Razorpay keys, Setu keys

# 3. Run migrations
go run cmd/migrate/run.go

# 4. Start the server
go run cmd/api/run.go
```

The server starts on the port specified by `HTTP_PORT` (default: `5000`).

### Verify

```bash
curl http://localhost:5000/health
# → "ok"
```

---

## Configuration

Configuration is loaded from environment variables (with `.env` support via godotenv):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `5000` | Server listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `FIREBASE_CREDENTIALS_FILE` | `config/firebase-service-account.json` | Path to Firebase service account JSON |
| `FIREBASE_PROJECT_ID` | `myslotmate-25994` | Firebase project ID |
| `SETU_BASE_URL` | `https://uat.setu.co` | Setu OKYC API base URL |
| `SETU_CLIENT_ID` | — | Setu client ID |
| `SETU_CLIENT_SECRET` | — | Setu client secret |
| `SETU_PRODUCT_INSTANCE_ID` | — | Setu product instance ID |
| `RAZORPAY_KEY_ID` | — | Razorpay API key ID (required) |
| `RAZORPAY_KEY_SECRET` | — | Razorpay API key secret (required) |
| `RAZORPAY_ACCOUNT_NUMBER` | — | RazorpayX linked bank account number |
| `RAZORPAY_WEBHOOK_SECRET` | — | Razorpay webhook secret (for signature verification) |

---

## Migrations

SQL migration files are in `migrations/` and run in order:

| Migration | Description |
|-----------|-------------|
| `20260228120000_init_schema.sql` | Core schema — 10 tables, enums, triggers (accounts, users, hosts, events, bookings, reviews, payments, inbox, support, fraud) |
| `20260228130000_add_processing_status.sql` | Adds `processing` to `payment_status` enum |
| `20260228130001_earnings_payouts_schema.sql` | Adds `platform_settings`, `payout_methods`, `host_earnings` tables; extends `bookings` and `payments` with fee/payout columns |

```bash
go run cmd/migrate/run.go
```

---

## Graceful Shutdown

The server listens for `SIGINT` / `SIGTERM` and performs:

1. HTTP server shutdown (20s timeout)
2. Worker pool drain (processes remaining tasks)
3. Socket.IO server close
4. Database connection close

---

## License

Private — All rights reserved.
