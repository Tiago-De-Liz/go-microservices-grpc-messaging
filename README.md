# 🚀 Go Microservices with gRPC and Messaging

Educational project demonstrating microservices architecture in Go with synchronous (gRPC) and asynchronous (SQS/SNS-like) communication patterns.

> ⚠️ **Educational Purpose:** This project simulates production patterns locally. The message broker is an in-memory implementation that follows AWS SQS/SNS concepts.

## 📚 Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Communication Patterns](#communication-patterns)
- [AWS Mapping](#aws-mapping)

---

## Overview

This project demonstrates key concepts of distributed systems:

| Concept | Implementation |
|---------|---------------|
| **Microservices** | Independent services with clear boundaries |
| **Synchronous Communication** | gRPC with Protocol Buffers |
| **Asynchronous Communication** | In-memory message broker (SQS/SNS-like) |
| **Event-Driven Architecture** | Pub/Sub with fan-out |
| **Idempotency** | Duplicate message handling |
| **Retry Logic** | Exponential backoff |
| **Dead Letter Queue** | Failed message handling |

### Tech Stack

- **Language:** Go 1.21+
- **RPC:** gRPC with Protocol Buffers
- **HTTP:** Standard library (net/http)
- **Messaging:** Custom in-memory broker

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         EXTERNAL CLIENT                              │
└───────────────────────────────┬─────────────────────────────────────┘
                                │ HTTP
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      ORDER SERVICE (:8080)                           │
│  • HTTP API Gateway                                                  │
│  • Calls Payment via gRPC (sync)                                     │
│  • Publishes events (async)                                          │
└───────────────┬─────────────────────────────┬───────────────────────┘
                │ gRPC                        │ Events
                ▼                             ▼
┌───────────────────────────┐   ┌─────────────────────────────────────┐
│   PAYMENT SERVICE         │   │         MESSAGE BROKER               │
│   (:50051 gRPC)           │   │  ┌─────────────────────────────┐    │
│                           │   │  │ Topic: order.created        │    │
│   • Process payments      │   │  └────────────┬────────────────┘    │
│   • Validate cards        │   │               │ fan-out              │
│   • Return result         │   │  ┌────────────┴────────────┐        │
└───────────────────────────┘   │  ▼                         ▼        │
                                │ [Queue: notifications] [Queue: audit]│
                                └───────────┬─────────────┬───────────┘
                                            │             │
                                            ▼             ▼
                                    ┌─────────────┐ ┌─────────────┐
                                    │ Notification│ │   Audit     │
                                    │   Worker    │ │   Worker    │
                                    └─────────────┘ └─────────────┘
```

---

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Make (optional, for Makefile commands)

### Running the Services

**Terminal 1 - Payment Service (gRPC):**
```bash
go run ./services/payment/cmd
# Listening on :50051
```

**Terminal 2 - Order Service (HTTP + Workers):**
```bash
go run ./services/order/cmd
# Listening on :8080
```

### Testing the Flow

**Create an order:**
```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_email": "user@example.com",
    "items": [
      {
        "product_name": "Laptop Pro",
        "quantity": 1,
        "unit_price_cents": 249900
      }
    ]
  }'
```

**Expected flow:**
1. ✅ Order Service receives HTTP request
2. ✅ Order Service calls Payment via gRPC
3. ✅ Payment Service processes and returns
4. ✅ Order Service publishes `order.created` event
5. ✅ Notification Worker sends email (simulated)
6. ✅ Audit Worker logs the event

**Check the logs to see the complete flow!**

---

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/orders` | Create a new order |
| `GET` | `/orders` | List all orders |
| `GET` | `/orders/{id}` | Get order by ID |
| `GET` | `/health` | Health check |
| `GET` | `/stats` | Service statistics |

### Create Order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_email": "user@example.com",
    "items": [
      {
        "product_name": "Laptop Pro",
        "quantity": 1,
        "unit_price_cents": 249900
      }
    ]
  }'
```

**Response (201 Created):**
```json
{
  "id": "ord_abc123",
  "customer_email": "user@example.com",
  "items": [...],
  "total_cents": 249900,
  "status": 2,
  "payment_transaction_id": "tx_def456"
}
```

### Create Order with Multiple Items

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_email": "client@example.com",
    "items": [
      {"product_name": "Laptop", "quantity": 1, "unit_price_cents": 350000},
      {"product_name": "Mouse", "quantity": 2, "unit_price_cents": 15000}
    ]
  }'
```

### List All Orders

```bash
curl http://localhost:8080/orders
```

**Response:**
```json
{
  "orders": [...],
  "count": 3
}
```

### Get Order by ID

```bash
curl http://localhost:8080/orders/ord_abc123
```

### Health Check

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "order"
}
```

### Service Statistics

```bash
curl http://localhost:8080/stats
```

**Response:**
```json
{
  "TotalOrders": 5,
  "PaidOrders": 4,
  "CancelledOrders": 1,
  "PendingOrders": 0,
  "TotalRevenueCents": 899800
}
```

### Test Scenarios

```bash
# ✅ Successful payment
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_email":"test@example.com","items":[{"product_name":"Book","quantity":1,"unit_price_cents":5000}]}'

# ❌ Payment declined (amounts ending in 99 cents are simulated as declined)
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_email":"test@example.com","items":[{"product_name":"Test","quantity":1,"unit_price_cents":199}]}'

# ❌ Missing email validation error
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"items":[{"product_name":"Test","quantity":1,"unit_price_cents":1000}]}'

# ❌ Empty items validation error
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_email":"test@example.com","items":[]}'
```

---

## Project Structure

```
go-microservices-grpc-messaging/
├── docs/                           # Documentation
│   ├── concepts.md                 # Conceptual explanations
│   ├── architecture.md             # Architecture details
│   └── interview-prep.md           # Interview questions & answers
│
├── proto/                          # Protocol Buffers & types
│   ├── payment/                    # Payment service types
│   └── order/                      # Order event types
│
├── pkg/                            # Shared packages
│   └── broker/                     # Message broker (SQS/SNS simulation)
│       ├── broker.go               # Main broker
│       ├── topic.go                # SNS-like topics
│       ├── queue.go                # SQS-like queues
│       ├── worker.go               # Queue consumers
│       └── message.go              # Message types
│
├── services/
│   ├── order/                      # Order Service (API Gateway)
│   │   ├── cmd/main.go             # Entry point
│   │   └── internal/
│   │       ├── handler/            # HTTP handlers
│   │       └── service/            # Business logic
│   │
│   └── payment/                    # Payment Service (gRPC)
│       ├── cmd/main.go             # Entry point
│       └── internal/
│           ├── server/             # gRPC server
│           └── service/            # Business logic
│
├── Makefile                        # Build commands
└── README.md                       # This file
```

---

## Communication Patterns

### When to Use gRPC (Synchronous)

| Use Case | Reason |
|----------|--------|
| Payment processing | Need immediate confirmation |
| User authentication | Can't proceed without result |
| Data validation | Blocking operation |
| Real-time queries | Low-latency requirement |

### When to Use Events (Asynchronous)

| Use Case | Reason |
|----------|--------|
| Notifications | Can be delayed |
| Audit logging | Non-blocking |
| Analytics | Fire-and-forget |
| Cross-service sync | Eventual consistency OK |

### This Project's Choices

| Communication | Path | Reason |
|---------------|------|--------|
| gRPC (sync) | Order → Payment | Need payment result to confirm order |
| Events (async) | Order → Notifications | Email doesn't block order creation |
| Events (async) | Order → Audit | Logging is fire-and-forget |

---

## AWS Mapping

| This Project | AWS Production |
|--------------|----------------|
| Order Service | ECS/EKS + ALB |
| Payment Service | ECS/EKS (internal) |
| Topic (`order.created`) | AWS SNS |
| Queue (`notifications`) | AWS SQS |
| Queue (`audit`) | AWS SQS |
| Workers | Lambda or ECS |
| gRPC internal | Service Mesh / App Mesh |

### Migration Path

The code architecture allows easy migration:

1. **Services:** Deploy as containers on ECS/EKS
2. **Broker:** Replace `pkg/broker` with AWS SDK calls
3. **Topics:** Create SNS topics with same names
4. **Queues:** Create SQS queues subscribed to SNS
5. **Workers:** Convert to Lambda or keep as ECS tasks

---

## License

MIT License - Use freely for learning and portfolio.

---

## Contributing

This is an educational project. Feel free to:
- Fork and extend
- Open issues for questions
- Submit PRs with improvements

---

Made with 💚 for learning distributed systems in Go.
