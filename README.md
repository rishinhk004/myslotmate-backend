# MySlotMate — Backend

**A production-grade event booking platform backend built with Go, following Clean Architecture and enterprise design patterns.**

MySlotMate allows users to discover and book event slots, and enables verified hosts to create and manage events, track earnings, and communicate with attendees — all backed by Aadhar-based identity verification, real-time WebSocket updates, and a robust payment/payout system.

---

## Table of Contents

- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Design Patterns](#design-patterns)
- [Project Structure](#project-structure)
- [Database Schema](#database-schema)
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
| **Business Logic** | `internal/service` | Core rules (overbooking prevention, fee split, verification flow), orchestrates repos + infra. |
| **Data Access** | `internal/repository` | Direct SQL operations. Abstraction over PostgreSQL; mockable for unit tests. |
| **Infrastructure** | `internal/lib/*` | Reusable components — Event Bus, Worker Pool, Identity (KYC), Real-time (Socket.IO). |

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

// Events: UserCreated, HostCreated, EventCreated, BookingCreated, BookingStatusChanged
dispatcher.Subscribe("booking_created", analyticsObserver)
dispatcher.Publish("booking_created", bookingData)
```

### 3. Strategy
**Where:** `internal/lib/identity/aadhar_provider.go`  
**Why:** Swap KYC providers (Setu production, mock for testing) without changing `UserService`.

```go
type AadharProvider interface {
    InitiateVerification(ctx context.Context, aadharNumber string) (transactionID string, err error)
    VerifyOTP(ctx context.Context, transactionID, otp string) (*AadharVerificationResult, error)
}
```

Implementations: `SetuAadharProvider` (production via Setu OKYC API).

### 4. Repository
**Where:** `internal/repository/*.go`  
**Why:** Abstracts database access behind interfaces, enabling service-layer unit tests with mocked repositories.

Repositories: `UserRepository`, `HostRepository`, `EventRepository`, `BookingRepository`, `ReviewRepository`, `InboxRepository`.

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

```
main() → Config → DB → Dispatcher → WorkerPool → Firebase
       → Repositories → Identity Provider → Services → Controllers → Router → HTTP Server
```

---

## Project Structure

```
myslotmate-backend/
├── cmd/
│   ├── api/run.go              # Application entry point & DI wiring
│   ├── checkdb/run.go          # DB connectivity check utility
│   └── migrate/run.go          # Migration runner
├── config/
│   └── firebase-service-account.json
├── docs/
│   ├── ARCHITECTURE.md         # Detailed architecture document
│   └── SCHEMA.md               # Full schema HLD with field-level detail
├── internal/
│   ├── auth/
│   │   └── handler.go          # Firebase ID token verification
│   ├── config/
│   │   └── config.go           # Env-based configuration loader
│   ├── controller/             # HTTP handlers (transport layer)
│   │   ├── booking_controller.go
│   │   ├── event_controller.go
│   │   ├── host_controller.go
│   │   ├── inbox_controller.go
│   │   ├── response.go         # Standardized JSON response helpers
│   │   ├── review_controller.go
│   │   └── user_controller.go
│   ├── db/
│   │   └── db.go               # PostgreSQL connection (pgx)
│   ├── firebase/
│   │   └── firebase.go         # Firebase Admin SDK initialization
│   ├── lib/
│   │   ├── event/
│   │   │   └── dispatcher.go   # Singleton Observer (event bus)
│   │   ├── identity/
│   │   │   ├── aadhar_provider.go  # Strategy interface
│   │   │   └── setu_provider.go    # Setu OKYC implementation
│   │   ├── realtime/
│   │   │   └── socket_service.go   # Socket.IO server
│   │   └── worker/
│   │       └── pool.go         # Background worker pool (executor)
│   ├── models/                 # Domain structs & enums
│   │   ├── account.go
│   │   ├── booking.go
│   │   ├── enums.go
│   │   ├── event.go
│   │   ├── fraud.go
│   │   ├── host_earnings.go
│   │   ├── host.go
│   │   ├── inbox.go
│   │   ├── payment.go
│   │   ├── payout_method.go
│   │   ├── platform_settings.go
│   │   ├── review.go
│   │   ├── support.go
│   │   └── user.go
│   ├── repository/             # Data access layer (SQL)
│   │   ├── booking_repository.go
│   │   ├── event_repository.go
│   │   ├── host_repository.go
│   │   ├── inbox_repository.go
│   │   ├── review_repository.go
│   │   └── user_repository.go
│   ├── server/
│   │   └── router.go           # Chi router, middleware, route mounting
│   └── service/                # Business logic layer
│       ├── booking_service.go
│       ├── event_service.go
│       ├── host_service.go
│       ├── inbox_service.go
│       ├── review_service.go
│       └── user_service.go
└── migrations/                 # PostgreSQL migration files
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

#### `accounts`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `owner_type` | ENUM | `user` \| `host` |
| `owner_id` | UUID | UNIQUE per `(owner_type, owner_id)` |
| `balance_cents` | BIGINT | CHECK `≥ 0` |
| `bank_details` | JSONB | |

> **Auto-created** via triggers on `users` and `hosts` insert.

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

> **Overbooking prevention:** Service layer checks `SUM(quantity) WHERE status IN ('pending','confirmed') < event.capacity` before confirming.

#### `payments`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `idempotency_key` | VARCHAR | UNIQUE |
| `account_id` | UUID | FK → `accounts` |
| `type` | ENUM | `booking` \| `withdrawal` \| `refund` \| `payout` \| `topup` |
| `reference_id` | UUID | |
| `amount_cents` | BIGINT | |
| `status` | ENUM | `pending` \| `processing` \| `completed` \| `failed` \| `reversed` |
| `payout_method_id` | UUID | FK → `payout_methods` |
| `display_reference` | VARCHAR | Human-readable (e.g. `TXN-88234`) |
| `retry_count` | INT | DEFAULT `0` |
| `last_error` | TEXT | |

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

#### `payout_methods`
| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `host_id` | UUID | FK → `hosts` |
| `type` | ENUM | `bank` \| `upi` |
| `bank_name` | VARCHAR | |
| `account_type` | VARCHAR | `checking` \| `savings` |
| `last_four_digits` | VARCHAR | Masked |
| `upi_id` | VARCHAR | |
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
| `blocked_until` | TIMESTAMPTZ | |
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
| `GET` | `/hosts/me?user_id={uuid}` | Get own host profile |

### Events

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/events/` | Create a new event (host only) |
| `GET` | `/events/host/{hostID}` | List all events for a host |

### Bookings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/bookings/` | Book tickets for an event (overbooking-safe) |
| `GET` | `/bookings/user/{userID}` | Get booking history for a user |

<details>
<summary>Request Example</summary>

**POST `/bookings/`**
```json
{
  "user_id": "uuid",
  "event_id": "uuid",
  "quantity": 2
}
// → 201 (auto-calculates amount_cents, service_fee_cents, net_earning_cents)
```
</details>

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

### Setup

```bash
# 1. Clone
git clone <repo-url>
cd myslotmate-backend

# 2. Configure environment
cp .env.example .env
# Edit .env with your database URL, Firebase config, Setu keys

# 3. Run migrations
go run cmd/migrate/run.go

# 4. Start the server
go run cmd/api/run.go
```

The server starts on the port specified by `HTTP_PORT` (default: `8080`).

### Verify

```bash
curl http://localhost:8080/health
# → "ok"
```

---

## Configuration

Configuration is loaded from environment variables (with `.env` support via godotenv):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | Server listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `FIREBASE_CREDENTIALS_FILE` | `config/firebase-service-account.json` | Path to Firebase service account JSON |
| `FIREBASE_PROJECT_ID` | `myslotmate-25994` | Firebase project ID |
| `SETU_BASE_URL` | `https://uat.setu.co` | Setu OKYC API base URL |
| `SETU_CLIENT_ID` | — | Setu client ID |
| `SETU_CLIENT_SECRET` | — | Setu client secret |
| `SETU_PRODUCT_INSTANCE_ID` | — | Setu product instance ID |

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

## Key Business Logic

### Overbooking Prevention
Before confirming a booking, the service checks:
```
SUM(quantity) WHERE event_id = ? AND status IN ('pending', 'confirmed') + new_quantity ≤ event.capacity
```

### Platform Fee Split
Every booking automatically calculates:
- **Service fee** = 15% of booking amount → platform
- **Net earning** = 85% of booking amount → host

### Aadhar Verification Flow
```
User → POST /auth/verify-aadhar/init (aadhar number)
     ← { transaction_id }
User → POST /auth/verify-aadhar/complete (transaction_id + OTP)
     ← User marked as verified → can now become a Host
```

### Graceful Shutdown
The server listens for `SIGINT` / `SIGTERM` and performs:
1. HTTP server shutdown (20s timeout)
2. Worker pool drain (processes remaining tasks)
3. Socket.IO server close
4. Database connection close

---

## License

Private — All rights reserved.