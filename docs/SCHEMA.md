# MySlotMate – Database Schema (HLD)

This document defines the high-level data model for MySlotMate, aligned with the DB flow diagram and functional requirements. **Anyone can become a host after Aadhar verification**; the schema supports that flow with `User.is_verified` and a `Host` profile linked to a verified user.

---

## Entity Relationship Overview

```
User (1) ─────────────────┬────────────────── (1) Host
   │                      │
   │ is_verified          │ user_id
   │ account_id           │ account_id
   │                      │
   ▼                      ▼
Account (1) ◄─────────────┴────────────── (1)
   │
   │ payments, withdrawals, bank_details
   ▼
Payment / Transaction

Event (many) ◄── host_id ─── Host
   │
   ├── Booking (many) ──► User
   ├── Review (many) ──► User
   ├── InboxMessage (many) ──► Host (broadcasts)
   └── capacity / max_tickets (overbooking)

SupportTicket (many) ◄── user_id ── User / Host
FraudFlag (many) ◄── user_id ── User
```

---

## 1. User

Represents any registered person. After **login**, they can complete **Aadhar verification**. Only **verified** users can register as a Host.

| Field            | Type     | Description |
|------------------|----------|-------------|
| `_id`            | UUID     | Primary key. |
| `name`           | STRING   | Display name. |
| `phn_number`     | STRING   | Phone number. |
| `email`          | STRING   | From auth (e.g. Firebase). |
| `account_details`| `_acc_id`| FK → Account (payment/bank details). |
| **`is_verified`**| BOOLEAN  | **Aadhar verification done (required to become host).** |
| `verified_at`    | TIMESTAMP| When verification was completed (optional). |
| `auth_uid`       | STRING   | Firebase (or other IdP) UID. |

**Notes:**  
- Registration → create User (e.g. from `/auth/signUp`).  
- Profile update: `name`, `phn_number`, `account_details`.  
- Booking history: query `Booking` by `user_id`.

---

## 2. Host

A **verified user** who can create events, see analytics, use inbox, withdraw payments, and contact support. One User can have at most one Host profile.

| Field             | Type     | Description |
|-------------------|----------|-------------|
| `_id`             | UUID     | Primary key. |
| **`user_id`**     | UUID     | **FK → User. Only users with `is_verified = true` can have a Host.** |
| `name`            | STRING   | Host display name (can mirror or override User). |
| `phn_number`      | STRING   | Contact number. |
| `account_details` | `_acc_id`| FK → Account (payouts, bank details). |
| `created_at`      | TIMESTAMP| When host profile was created. |
| `updated_at`      | TIMESTAMP| Last profile update. |

**Notes:**  
- Host can update profile: `name`, `phn_number`, `account_details`.  
- Calendar: derived from `Event` where `host_id = _id` (filter by date range).  
- Inbox: `InboxMessage` where `host_id = _id`.  
- Withdrawals & payment history: `Payment` where `account_id` = host’s account (or payee = host).  
- Support: `SupportTicket` where `user_id` = Host’s `user_id` (or a dedicated `host_id` if you prefer).

---

## 3. Account (Wallet)

One account per user or host. Stores wallet balance, payment identity, and bank details.

| Field            | Type     | Description |
|------------------|----------|-------------|
| `_id`            | UUID     | Primary key. |
| `owner_type`     | ENUM     | `user` \| `host`. |
| `owner_id`       | UUID     | User._id or Host._id. |
| **`balance_cents`** | BIGINT | Wallet balance (≥ 0). Updated on topup, booking, refund, payout. |
| `bank_details`   | JSON/OBJ | Account number (masked), IFSC, UPI, etc. |
| `created_at`     | TIMESTAMP| |
| `updated_at`     | TIMESTAMP| |

**Constraints:** `UNIQUE(owner_type, owner_id)` — one account per owner.

**Notes:**  
- Auto-created on User/Host insert when `account_id` is null.  
- Topup: `Payment` type `topup` → credit `balance_cents`.  
- Booking: debit user wallet; payout: credit host wallet.  
- Withdraw: create `Payment` type `withdrawal` and status flow + idempotency.

---

## 4. Event

Created and managed by a Host. Has a capacity to **prevent overbooking**; bookings are stored in the **Booking** entity.

| Field           | Type       | Description |
|-----------------|------------|-------------|
| `_id`           | UUID       | Primary key. |
| `name`          | STRING     | Event name. |
| `time`          | TIMESTAMP  | Event date/time (or start time). |
| `end_time`      | TIMESTAMP  | Optional. |
| **`host_id`**   | UUID       | **FK → Host.** |
| **`capacity`**  | INT        | **Max tickets/attendees (for overbooking prevention).** |
| `users`         | `[_user_id]`| **Deprecated in favor of Booking.** Prefer: “attendees” = distinct User IDs from confirmed Bookings. |
| `reviews`       | `[_review_id]`| FK array → Review. |
| `ai_suggestion` | STRING     | AI-generated suggestions for the host. |
| `created_at`    | TIMESTAMP  | |
| `updated_at`    | TIMESTAMP  | |

**Notes:**  
- Host can create/update/delete event.  
- Analytics: aggregate over Event (views, bookings, revenue, reviews, sentiment).  
- Overbooking: sum of confirmed Booking quantities per event must not exceed `Event.capacity`.

---

## 5. Booking

**New entity.** Supports: search events → book tickets, prevent overbooking, cancel booking, status tracking.

| Field           | Type     | Description |
|-----------------|----------|-------------|
| `_id`           | UUID     | Primary key. |
| `event_id`      | UUID     | FK → Event. |
| `user_id`       | UUID     | FK → User. |
| **`quantity`**  | INT      | Number of tickets. |
| **`status`**    | ENUM     | `pending` \| `confirmed` \| `cancelled` \| `refunded`. |
| `payment_id`    | UUID     | FK → Payment (optional; link to payment that confirmed the booking). |
| `idempotency_key`| STRING  | Optional; for duplicate booking requests. |
| `amount_cents`  | BIGINT   | Total booking value. |
| `service_fee_cents` | BIGINT | Platform fee (15%). |
| `net_earning_cents` | BIGINT | Host net (85%). |
| `created_at`    | TIMESTAMP| |
| `updated_at`    | TIMESTAMP| |
| `cancelled_at`  | TIMESTAMP| Set when status → cancelled. |

**Notes:**  
- User books: create Booking (e.g. `pending`) → verify payment → set `confirmed` and set `payment_id`.  
- Overbooking: before confirming, check `SUM(quantity)` for `event_id` where `status IN ('pending','confirmed')` < `Event.capacity`.  
- Cancel: set `status = cancelled`, `cancelled_at = now`; optionally trigger refund and idempotent payment reversal.  
- Booking history: list Bookings by `user_id` (and optionally filter by `status`).

---

## 6. Review

User-written reviews for an event; used for **AI**: analyze reviews, host suggestions, sentiment score.

| Field         | Type      | Description |
|---------------|-----------|-------------|
| `_id`         | UUID      | Primary key. |
| `event_id`    | UUID      | FK → Event (reviews belong to event). |
| **`user_id`** | UUID      | **FK → User (who wrote the review).** |
| `name`        | STRING    | Optional display name or title. |
| `description` | STRING    | Review text. |
| `reply`       | [STRING]  | Array of replies (e.g. host replies). |
| `sentiment_score` | FLOAT | **Optional; AI-generated sentiment score.** |
| `created_at`  | TIMESTAMP | |
| `updated_at`  | TIMESTAMP | |

**Notes:**  
- Event’s `reviews` array can store `_review_id` for quick access or derive from `Review.event_id`.  
- AI: analyze `description` (and optionally `reply`), compute sentiment, generate `Event.ai_suggestion`.

---

## 7. Payment (Transaction)

**New entity.** Supports: verify payment, retry on failure, **idempotency** for duplicate requests.

| Field             | Type     | Description |
|-------------------|----------|-------------|
| `_id`             | UUID     | Primary key. |
| **`idempotency_key`** | STRING | **Unique key per client request (e.g. booking_id + request_id).** |
| `account_id`      | UUID     | FK → Account (payer or payee depending on type). |
| `type`            | ENUM     | `booking` \| `withdrawal` \| `refund` \| `payout` \| `topup`. |
| `reference_id`    | UUID     | e.g. Booking._id for type=booking. |
| `amount_cents`    | INT      | Amount in smallest currency unit. |
| **`status`**      | ENUM     | `pending` \| `processing` \| `completed` \| `failed` \| `reversed`. |
| `payout_method_id`| UUID     | FK → PayoutMethod (for type=payout). |
| `display_reference` | STRING | Human-readable ref (e.g. TXN-88234). |
| `retry_count`     | INT      | Number of payment retries. |
| `last_error`      | STRING   | Last failure reason (for retry). |
| `created_at`      | TIMESTAMP| |
| `updated_at`      | TIMESTAMP| |

**Notes:**  
- Verify payment: check `status == completed` and match `reference_id` (e.g. booking).  
- Retry: increment `retry_count`, update `status`/`last_error` on failure.  
- Duplicate requests: reject or return existing payment when `idempotency_key` already exists.

---

## 8. InboxMessage (Event Update)

**New entity.** Host broadcasts updates per event (inbox = updates for each event).

| Field        | Type     | Description |
|--------------|----------|-------------|
| `_id`        | UUID     | Primary key. |
| `event_id`   | UUID     | FK → Event. |
| `host_id`    | UUID     | FK → Host (sender). |
| `message`    | STRING   | Broadcast content. |
| `created_at` | TIMESTAMP| |

**Notes:**  
- Recipients: users with confirmed Booking for that `event_id` (or all who ever booked).  
- Host “checks inbox” = list InboxMessages by `host_id` (and optionally by `event_id`).

---

## 9. SupportTicket

**New entity.** Host (or User) can connect with support team.

| Field        | Type     | Description |
|--------------|----------|-------------|
| `_id`        | UUID     | Primary key. |
| `user_id`    | UUID     | FK → User (or Host’s user_id). |
| `subject`    | STRING   | |
| `messages`   | [OBJ]    | Array of { sender, text, created_at }. |
| `status`     | ENUM     | `open` \| `in_progress` \| `resolved` \| `closed`. |
| `created_at` | TIMESTAMP| |
| `updated_at` | TIMESTAMP| |

---

## 10. FraudFlag (Suspicious User / Block)

**New entity.** For fraud: abnormal booking spikes, payment abuse, block suspicious users.

| Field         | Type     | Description |
|---------------|----------|-------------|
| `_id`         | UUID     | Primary key. |
| `user_id`     | UUID     | FK → User. |
| `type`        | ENUM     | `abnormal_booking_spike` \| `payment_abuse` \| `suspicious_activity` \| `manual_block`. |
| `reason`      | STRING   | Optional description. |
| `blocked_at`  | TIMESTAMP| |
| `blocked_until`| TIMESTAMP| Optional; null = indefinite. |
| `is_active`   | BOOLEAN  | If true, user is blocked from booking/payment as per policy. |

**Notes:**  
- Detect abnormal booking spikes: aggregate bookings per user/time window; create FraudFlag when threshold exceeded.  
- Detect payment abuse: failed/retry patterns, chargebacks; create FraudFlag.  
- Block suspicious users: set `is_active = true`; auth/booking/payment layers check FraudFlag before allowing actions.

---

## 11. PayoutMethod (Earnings & Payouts)

**New entity.** Multiple payout methods per host (bank accounts, UPI IDs).

| Field               | Type     | Description |
|---------------------|----------|-------------|
| `_id`               | UUID     | Primary key. |
| `host_id`           | UUID     | FK → Host. |
| `type`              | ENUM     | `bank` \| `upi`. |
| `bank_name`         | STRING   | For type=bank (e.g. Canara Bank). |
| `account_type`      | STRING   | For bank: `checking` \| `savings`. |
| `last_four_digits`  | STRING   | Masked display (e.g. 4567). |
| `upi_id`            | STRING   | For type=upi (e.g. user@okdbs). |
| `is_verified`       | BOOLEAN  | Whether method is verified. |
| `is_primary`       | BOOLEAN  | Default payout method. |
| `created_at`        | TIMESTAMP| |
| `updated_at`        | TIMESTAMP| |

**Notes:**  
- Host can add/edit/remove payout methods via "Manage" / "+ Add new method".  
- Payout history references `payout_method_id` on Payment.

---

## 12. HostEarnings (Earnings Summary)

**New entity.** Aggregate earnings per host for the Earnings & Payouts dashboard.

| Field                    | Type     | Description |
|--------------------------|----------|-------------|
| `_id`                    | UUID     | Primary key. |
| `host_id`                | UUID     | FK → Host (unique). |
| `total_earnings_cents`   | BIGINT   | Lifetime earnings. |
| `pending_clearance_cents`| BIGINT   | Funds not yet available for payout. |
| `estimated_clearance_at` | TIMESTAMP| When pending becomes available. |
| `created_at`             | TIMESTAMP| |
| `updated_at`             | TIMESTAMP| |

**Notes:**  
- **Available balance** comes from `Account.balance_cents` (host’s account).  
- **Total earnings** = `host_earnings.total_earnings_cents`.  
- **Pending clearance** = `host_earnings.pending_clearance_cents` + `estimated_clearance_at`.  
- Monthly trend (e.g. -12%) computed from Payment/Booking history by month.

---

## 13. PlatformSettings (Fee Breakdown)

**New entity.** Config for platform fee (e.g. 85% host, 15% platform).

| Field       | Type     | Description |
|-------------|----------|-------------|
| `_id`       | UUID     | Primary key. |
| `key`       | STRING   | e.g. `platform_fee`. |
| `value`     | JSONB    | e.g. `{"host_percentage": 85, "platform_percentage": 15}`. |
| `created_at`| TIMESTAMP| |
| `updated_at`| TIMESTAMP| |

**Notes:**  
- Used for "Platform Fee Breakdown" (Average Booking Value, Service Fee, Net Earning).  
- Per-booking breakdown: `Booking.amount_cents`, `service_fee_cents`, `net_earning_cents`.

---

## Summary: Requirements → Entities/Fields

| Requirement | Schema mapping |
|-------------|-----------------|
| User register / login | User + Firebase `auth_uid`; Auth via existing `/auth/signUp`. |
| User update profile | User: name, phn_number, account_details. |
| User view booking history | Booking by `user_id`. |
| Aadhar verify → can become host | User.`is_verified`; Host allowed only if User.`is_verified`. |
| Host create/update/delete event | Event; host_id = Host._id. |
| Host see analytics | Aggregations on Event, Booking, Payment, Review. |
| Host update profile | Host: name, phn_number, account_details. |
| Host check calendar | Event by host_id and date range. |
| Host broadcast inbox | InboxMessage by host_id (and event_id). |
| Host withdraw, payment history, bank details | Account + Payment (type payout); PayoutMethod; HostEarnings. |
| Host earnings summary (total, available, pending) | HostEarnings + Account.balance_cents. |
| Host payout methods (bank, UPI) | PayoutMethod. |
| Platform fee breakdown (85/15) | PlatformSettings; Booking.amount_cents, service_fee_cents, net_earning_cents. |
| Host connect support | SupportTicket with user_id (host’s user). |
| User search events | Query Event (filters, date, host, etc.). |
| User book tickets | Create Booking; link Payment; enforce capacity. |
| Prevent overbooking | Event.capacity vs SUM(Booking.quantity) where status in (pending, confirmed). |
| User cancel booking | Booking.status = cancelled; optional refund. |
| Booking status tracking | Booking.status. |
| Verify payment | Payment.status = completed; reference_id. |
| Retry payment | Payment.retry_count, status, last_error. |
| Idempotency | Payment.idempotency_key; Booking.idempotency_key optional. |
| AI: analyze reviews, suggestions, sentiment | Review.description/reply; Event.ai_suggestion; Review.sentiment_score. |
| Fraud: abnormal booking spikes | FraudFlag.type = abnormal_booking_spike. |
| Fraud: payment abuse | FraudFlag.type = payment_abuse. |
| Block suspicious users | FraudFlag.is_active; enforce in auth/booking/payment. |

---

## Suggested Implementation Order

1. **User** (with `is_verified`), **Account**, **Host** (with `user_id` and check `User.is_verified`).
2. **Event** (with `host_id`, `capacity`).
3. **Booking** (with status, quantity, event_id, user_id); enforce capacity in service layer.
4. **Payment** (with idempotency_key, status, retry_count).
5. **Review** (with optional sentiment_score); wire AI to Event.ai_suggestion.
6. **InboxMessage**, **SupportTicket**, **FraudFlag**.

This schema keeps your existing DB flow (User, Host, Event, Account, Review) and adds the entities and fields needed for verification, booking, payments, AI, and fraud as per your requirements.
