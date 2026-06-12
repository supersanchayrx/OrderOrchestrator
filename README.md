# OrderOrchestrator

OrderOrchestrator is a Go service for owning the lifecycle of an order from creation to terminal states like delivered, cancelled, payment failed, or refunded.

The first implementation is a dependency-free MVP that compiles and runs locally while preserving the architecture from the design doc:

- Order state machine with valid transition enforcement
- REST API for creating orders, reading orders, reading history, transitions, and cancellation
- Idempotency hook for retry-safe mutating requests
- Append-only transition history
- Timeout worker for unpaid / stuck payment states
- Live order updates over Server-Sent Events
- Interfaces ready for Postgres, Redis, and Kafka adapters
- SQL migrations and Docker Compose infrastructure for the production-shaped version

## Run Locally

```powershell
go run ./cmd/server
```

The API listens on `http://localhost:8080`.

Open the minimal web app at:

```text
http://localhost:8080
```

## API Quick Start

Create an order:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/v1/orders `
  -ContentType 'application/json' `
  -Body '{"customerId":"customer-1","totalAmount":"499.00","currency":"INR","items":[{"skuId":"sku-1","quantity":1,"unitPrice":"499.00"}]}'
```

Move it to payment pending:

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/api/v1/orders/<order-id>/transition `
  -Headers @{"Idempotency-Key"="demo-key-1"} `
  -ContentType 'application/json' `
  -Body '{"targetState":"PAYMENT_PENDING","triggeredBy":"USER"}'
```

Stream live updates:

```powershell
curl.exe -N http://localhost:8080/api/v1/orders/<order-id>/stream
```

## Next Build Steps

1. Replace `MemoryRepository` with a Postgres repository backed by `migrations/`.
2. Replace `MemoryStore` with Redis using `SETNX` + TTL semantics.
3. Replace `LogPublisher` with Kafka on topic `order.events`.
4. Add WebSocket support if two-way rider/customer communication becomes necessary; SSE is enough for one-way order status tracking.
