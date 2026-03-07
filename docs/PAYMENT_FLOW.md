# MySlotMate – Payment & Wallet Flow

This document explains the wallet-based payment system, money flow, and financial entities in the MySlotMate backend.

---

## Architecture: Wallet-Based System

MySlotMate uses an **internal wallet** model (similar to Paytm, Ola, etc.). Users and hosts each have a wallet (`Account`) with a `balance_cents` field. All money movement happens **wallet-to-wallet**, with external transfers only at the edges (topup and payout).

```
External  ──topup──►  User Wallet  ──booking──►  Host Wallet  ──payout──►  Bank/UPI
                      (Account)                   (Account)                (PayoutMethod)
```

---

## Core Financial Entities

### 1. Account (Wallet)

**File:** `internal/models/account.go`  
**Table:** `accounts`

One wallet per user + one wallet per host. Auto-created via DB triggers on insert.

| Field          | Type   | Description                                  |
|----------------|--------|----------------------------------------------|
| `id`           | UUID   | Primary key                                  |
| `owner_type`   | ENUM   | `user` or `host`                             |
| `owner_id`     | UUID   | FK → User or Host                            |
| `balance_cents`| BIGINT | Wallet balance (≥ 0, enforced by CHECK)      |
| `bank_details` | JSONB  | Masked account number, IFSC, UPI, beneficiary|

**Constraints:**
- `UNIQUE(owner_type, owner_id)` — one wallet per owner
- `CHECK (balance_cents >= 0)` — wallet can never go negative

**Auto-creation triggers** (in `migrations/20260228120000_init_schema.sql`):
- `create_account_on_user_insert` — creates `owner_type='user'` account with `balance_cents=0`
- `create_account_on_host_insert` — creates `owner_type='host'` account with `balance_cents=0`

---

### 2. Payment (Transaction Ledger)

**File:** `internal/models/payment.go`  
**Table:** `payments`

Every wallet movement is recorded as a `Payment`. This is the **single source of truth** for all money transactions.

| Field              | Type   | Description                                      |
|--------------------|--------|--------------------------------------------------|
| `id`               | UUID   | Primary key                                      |
| `idempotency_key`  | TEXT   | Unique key to prevent duplicate transactions     |
| `account_id`       | UUID   | FK → Account (payer or payee)                    |
| `type`             | ENUM   | `booking`, `withdrawal`, `refund`, `payout`, `topup` |
| `reference_id`     | UUID   | e.g. Booking ID for type=booking                 |
| `amount_cents`     | BIGINT | Transaction amount                               |
| `status`           | ENUM   | `pending`, `processing`, `completed`, `failed`, `reversed` |
| `retry_count`      | INT    | Number of retry attempts                         |
| `last_error`       | TEXT   | Last failure reason                              |
| `payout_method_id` | UUID   | FK → PayoutMethod (for type=payout)              |
| `display_reference`| TEXT   | Human-readable ref (e.g. `TXN-88234`)           |

**Payment Types:**

| Type         | Direction              | Description                       |
|--------------|------------------------|-----------------------------------|
| `topup`      | External → User wallet | User adds money to wallet         |
| `booking`    | User wallet → Platform | User pays for event tickets       |
| `refund`     | Platform → User wallet | Reversed booking payment          |
| `payout`     | Platform → Host bank   | Host withdraws to bank/UPI       |
| `withdrawal` | Host wallet → External | Host cashes out                   |

**Payment Status Lifecycle:**

```
pending ──► processing ──► completed
                       └──► failed (retry_count++)
completed ──► reversed (for refunds)
```

**Safety Features:**
- **Idempotency:** `idempotency_key` (unique) prevents duplicate charges from retried requests
- **Retry tracking:** `retry_count` + `last_error` for failed payment recovery
- **Display reference:** Human-readable `display_reference` like `TXN-88234`

---

### 3. Booking (with Fee Breakdown)

**File:** `internal/models/booking.go`  
**Table:** `bookings`

Each booking stores the full fee breakdown alongside status and quantity.

| Field               | Type   | Description                              |
|---------------------|--------|------------------------------------------|
| `id`                | UUID   | Primary key                              |
| `event_id`          | UUID   | FK → Event                               |
| `user_id`           | UUID   | FK → User                                |
| `quantity`          | INT    | Number of tickets booked                 |
| `status`            | ENUM   | `pending`, `confirmed`, `cancelled`, `refunded` |
| `payment_id`        | UUID   | FK → Payment (links to confirming payment)|
| `idempotency_key`   | TEXT   | Duplicate booking prevention             |
| `amount_cents`      | BIGINT | **Total booking value**                  |
| `service_fee_cents` | BIGINT | **Platform fee (15%)**                   |
| `net_earning_cents` | BIGINT | **Host net earning (85%)**               |
| `cancelled_at`      | TIMESTAMP | Set when status → cancelled           |

**Fee Calculation** (in `internal/service/booking_service.go`):

```go
totalAmount := pricePerTicket * quantity
serviceFee  := totalAmount * 0.15   // 15% platform cut
netEarning  := totalAmount - serviceFee  // 85% to host
```

---

### 4. HostEarnings (Earnings Dashboard)

**File:** `internal/models/host_earnings.go`  
**Table:** `host_earnings`

Aggregate earnings per host. Auto-created via DB trigger when a host is inserted.

| Field                     | Type      | Description                          |
|---------------------------|-----------|--------------------------------------|
| `id`                      | UUID      | Primary key                          |
| `host_id`                 | UUID      | FK → Host (unique)                   |
| `total_earnings_cents`    | BIGINT    | Lifetime earnings from all bookings  |
| `pending_clearance_cents` | BIGINT    | Funds earned but not yet withdrawable|
| `estimated_clearance_at`  | TIMESTAMP | When pending funds become available  |

**Dashboard Balance Computation:**

```
Available balance  = Account.balance_cents       (host's wallet)
Total earnings     = HostEarnings.total_earnings_cents
Pending clearance  = HostEarnings.pending_clearance_cents
```

---

### 5. PayoutMethod (Bank/UPI for Withdrawals)

**File:** `internal/models/payout_method.go`  
**Table:** `payout_methods`

Hosts can register multiple payout methods. When requesting a payout, the `Payment` record references `payout_method_id`.

| Field                      | Type    | Description                           |
|----------------------------|---------|---------------------------------------|
| `id`                       | UUID    | Primary key                           |
| `host_id`                  | UUID    | FK → Host                            |
| `type`                     | ENUM    | `bank` or `upi`                      |
| `bank_name`                | TEXT    | For bank type                         |
| `account_type`             | TEXT    | `checking` or `savings`              |
| `last_four_digits`         | TEXT    | Masked display (e.g. `**** 4567`)    |
| `account_number_encrypted` | TEXT    | Encrypted (never exposed in JSON)    |
| `ifsc`                     | TEXT    | Bank IFSC code                        |
| `beneficiary_name`         | TEXT    | Account holder name                   |
| `upi_id`                   | TEXT    | For UPI type (e.g. `user@okhdfc`)    |
| `is_verified`              | BOOL   | Whether method is verified            |
| `is_primary`               | BOOL   | Default payout method                 |

**DB Constraint:** `payout_method_bank_check` enforces required fields per type:
- Bank: requires `bank_name` + `last_four_digits`
- UPI: requires `upi_id`

---

### 6. PlatformSettings (Fee Configuration)

**File:** `internal/models/platform_settings.go`  
**Table:** `platform_settings`

Configurable platform fee split. Seeded with default values in migration.

| Field   | Type | Description                                           |
|---------|------|-------------------------------------------------------|
| `key`   | TEXT | Setting name (e.g. `platform_fee`)                   |
| `value` | JSONB| Setting value (e.g. `{"host_percentage": 85, "platform_percentage": 15}`) |

**Default seed** (in `migrations/20260228130001_earnings_payouts_schema.sql`):
```sql
INSERT INTO platform_settings (key, value) VALUES
  ('platform_fee', '{"host_percentage": 85, "platform_percentage": 15}')
ON CONFLICT (key) DO NOTHING;
```

---

## End-to-End Money Flows

### Flow 1: User Tops Up Wallet

```
1. User initiates topup via external gateway (Razorpay, etc.)
2. Payment record created: type=topup, status=pending
3. Gateway confirms → status=completed
4. Account.balance_cents += amount
```

### Flow 2: User Books an Event

```
1. User sends POST /bookings with event_id and quantity
2. BookingService checks event capacity (overbooking prevention)
3. Fee calculated:
     total    = price × quantity
     fee      = total × 15%
     net      = total × 85%
4. Booking created with status=pending
5. Payment record created: type=booking, reference_id=booking.id
6. User Account.balance_cents -= total
7. Host Account.balance_cents += net
8. HostEarnings.total_earnings_cents += net
9. Booking status → confirmed, payment status → completed
10. BookingCreated event published (Observer pattern)
```

### Flow 3: User Cancels Booking (Refund)

```
1. Booking.status → cancelled, cancelled_at = now
2. Payment record created: type=refund, reference_id=booking.id
3. User Account.balance_cents += refund_amount
4. Host Account.balance_cents -= net_earning
5. HostEarnings adjusted
6. Payment status → completed
```

### Flow 4: Host Withdraws Earnings (Payout)

```
1. Host selects payout method (bank or UPI) and amount
2. Validate: amount ≤ Account.balance_cents
3. Payment record created: type=payout, payout_method_id=selected method
4. Host Account.balance_cents -= amount
5. External transfer initiated to bank/UPI
6. On success: Payment status → completed
7. On failure: Payment status → failed, retry_count++, last_error set
```

---

## Fraud Prevention

**File:** `internal/models/fraud.go`  
**Table:** `fraud_flags`

Guards the money flow by flagging/blocking suspicious users.

| Flag Type                  | Trigger                                        |
|----------------------------|------------------------------------------------|
| `abnormal_booking_spike`   | User books unusually high volume in short time |
| `payment_abuse`            | Repeated failed payments, chargebacks          |
| `suspicious_activity`      | General suspicious behavior                    |
| `manual_block`             | Admin manually blocks a user                   |

When `is_active = true`, the user should be blocked from booking and payment operations.

---

## Database Triggers Summary

| Trigger                            | On              | Action                                    |
|------------------------------------|-----------------|-------------------------------------------|
| `create_account_on_user_insert`    | User INSERT     | Auto-create user wallet (balance=0)       |
| `create_account_on_host_insert`    | Host INSERT     | Auto-create host wallet (balance=0)       |
| `create_earnings_on_host_insert`   | Host INSERT     | Auto-create HostEarnings row              |
| `trg_host_user_must_be_verified`   | Host INSERT/UPD | Reject if user not verified               |
| `*_updated_at`                     | Various UPDATE  | Auto-set `updated_at = now()`             |

---

## Implementation Status

| Component                              | Status                                             |
|----------------------------------------|----------------------------------------------------|
| Fee calculation (15/85 split)          | ✅ Implemented (hardcoded — should read PlatformSettings) |
| Ticket price on Event                  | ❌ No price field on Event yet (dummy value used)  |
| Booking creation with fee breakdown    | ✅ Working                                         |
| Payment record creation                | ⚠️ Schema ready, not wired in service layer        |
| Wallet balance debit/credit            | ⚠️ Schema + constraints ready, not wired in service|
| HostEarnings auto-creation             | ✅ Working via DB trigger                          |
| PayoutMethod CRUD                      | ⚠️ Schema ready, no controller/service yet         |
| Fraud checks before booking            | ⚠️ Schema ready, not enforced in service layer     |
| Idempotency enforcement                | ⚠️ Column exists, not checked in booking service   |
