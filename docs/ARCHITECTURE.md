# MySlotMate - System Architecture & Design

This document details the architectural decisions, design patterns, and data flow of the MySlotMate backend.

## 1. High-Level Architecture (Clean Architecture)

The project follows **Clean Architecture** principles, enforcing separation of concerns into distinct layers.

```mermaid
graph TD
    Client[Client App / Frontend] --> Router[Router (Chi)]
    Router --> Controller[Controller Layer]
    Controller --> Service[Service Layer (Business Logic)]
    
    subgraph Core Logic
    Service --> Repository[Repository Layer (Data Access)]
    Service --> Worker[Worker Pool (Async Tasks)]
    Service --> Dispatcher[Event Dispatcher (Observer)]
    end
    
    Repository --> DB[(PostgreSQL)]
    Worker --> Email[3rd Party Services (Email/S3)]
    Dispatcher --> Analytics[Analytics/Notifications]
```

### **Layers Breakdown**

1.  **Transport Layer (`internal/server`, `internal/controller`)**
    *   **Router:** Handles HTTP routing, middleware (Auth, Logging), and request parsing.
    *   **Controller:** Decodes JSON requests, validates input, calls the Service, and formats JSON responses. **No business logic here.**

2.  **Business Logic Layer (`internal/service`)**
    *   **Service:** Contains the core business rules (e.g., "User cannot double book", "Calculate fees").
    *   Orchestrates dependencies like Repositories, Worker Pools, and Event Dispatchers.

3.  **Data Access Layer (`internal/repository`)**
    *   **Repository:** Performs direct SQL database operations. Abstraction over the database implementation.

4.  **Infrastructure/Libs (`internal/lib`)**
    *   Shared utilities, custom libraries, and pattern implementations (Identity, Worker, Event, Real-time).

---

## 2. Design Patterns Implemented

We use established software design patterns to ensure scalability, maintainability, and testability.

### A. Singleton Pattern
**Usage:** `Database Connection`, `Helper Configuration`, `Event Dispatcher`
*   **Why:** We only need one instance of the database connection pool or event bus throughout the application lifecycle to manage resources efficiently.
*   **Location:** `cmd/api/main.go` (initialization), `internal/lib/event/dispatcher.go`.

### B. Strategy Pattern
**Usage:** `Identity Verification (Aadhar)`
*   **Why:** Allows switching between different verification providers (e.g., **Setu**, **Cashfree**, or a **Mock/Dummy** provider) without changing the core business logic in `UserService`. The service just relies on the `AadharProvider` interface.
*   **Location:** `internal/lib/identity/aadhar_provider.go` (Interface), `setu_provider.go` (Implementation).

### C. Observer Pattern
**Usage:** `Event Bus (Internal Events)`
*   **Why:** Decouples services. When a `User` is created, the `UserService` publishes a `UserCreated` event. Other distinct parts of the system (like Email Service or Analytics) subscribe to this event. If we add a new "Welcome Notification" feature later, we don't touch `UserService` code; we just add a new subscriber.
*   **Location:** `internal/lib/event/dispatcher.go`.

### D. Executor / Worker Pool Pattern
**Usage:** `Background Tasks`
*   **Why:** Heavy operations (e.g., sending emails, image processing, complex calculations) should not block the main HTTP request thread. The Worker Pool allows us to submit tasks to a queue processed by background workers.
*   **Location:** `internal/lib/worker/pool.go`.

### E. Repository Pattern
**Usage:** `Database Access`
*   **Why:** Abstraction layer between the domain logic and the database. Makes it easier to write unit tests for Services by mocking the repository interface.
*   **Location:** `internal/repository/*.go`.

### F. Factory Pattern
**Usage:** `Dependency Injection`
*   **Why:** `NewUserService(...)`, `NewRepository(...)` functions encapsulate complex creation logic and dependency injection wiring.
*   **Location:** Constructors in all packages.

---

## 3. Schema Design Overview

The database is normalized to 3NF standards.

### User & Identity
*   **Users:** Core identity. (`id`, `email`, `is_verified`, `auth_uid`)
*   **Hosts:** Users who have verified identity and want to host events. (`id`, `user_id`, `name`)

### Events & Booking
*   **Events:** Created by Hosts. (`id`, `host_id`, `capacity`, `time`)
*   **Bookings:** Many-to-Many link between User and Event. (`id`, `event_id`, `user_id`, `status`)

### Engagement
*   **Reviews:** Optional feedback. (`id`, `booking_id`, `rating`, `comment`)
*   **Inbox/Messages:** Real-time communication. (`id`, `sender_id`, `room_id`, `content`)

---

## 4. API Flow Example: Aadhar Verification

This intricate flow demonstrates the interaction between multiple layers and patterns.

1.  **Request:** Client POSTs to `/auth/verify-aadhar/init` with Aadhar Number.
2.  **Controller:** `UserController` receives request, calls `UserService`.
3.  **Service:** `UserService` checks DB (via `UserRepository`) to ensure user isn't already verified.
4.  **Strategy:** `UserService` calls `AadharProvider.InitiateVerification`.
    *   If Prod: Uses `SetuAadharProvider` to call external API.
    *   If Dev: Uses `MockAadharProvider` (if configured).
5.  **Response:** Transaction ID returned to Client.
6.  **Next Step:** Client POSTs OTP to `/auth/verify-aadhar/complete`.
7.  **Service:** Validates OTP via `AadharProvider`.
8.  **Repository:** Calls `UserRepository.SetVerified(true)`.
9.  **Observer:** Publishes `UserVerified` event.
10. **Worker:** Background worker picks up event to send "Verification Successful" email.

---

## 5. Folder Structure Definition

*   **`cmd/`**: Entry points (main applications).
*   **`internal/`**: Private application code (not importable by other projects).
    *   **`controller/`**: HTTP Handlers.
    *   **`service/`**: Business Logic.
    *   **`repository/`**: Database Queries.
    *   **`models/`**: Data Structures (structs).
    *   **`lib/`**: Shared components (Identity, Realtime/Socket, Event, Worker).
    *   **`config/`**: Configuration loaders.
*   **`migrations/`**: SQL migration files.
