package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/pkg/broker"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/order"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/payment"
	"github.com/google/uuid"
)

type OrderService struct {
	mu            sync.RWMutex
	orders        map[string]*order.Order
	paymentClient payment.PaymentServiceClient
	broker        *broker.Broker
	topicName     string
}

func NewOrderService(
	paymentClient payment.PaymentServiceClient,
	b *broker.Broker,
	topicName string,
) *OrderService {
	return &OrderService{
		orders:        make(map[string]*order.Order),
		paymentClient: paymentClient,
		broker:        b,
		topicName:     topicName,
	}
}

type CreateOrderRequest struct {
	CustomerID    string
	CustomerEmail string
	Items         []order.OrderItem
	Currency      string
}

func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*order.Order, error) {
	if len(req.Items) == 0 {
		return nil, ErrNoItems
	}
	if req.CustomerEmail == "" {
		return nil, ErrMissingEmail
	}

	var totalCents int64
	for _, item := range req.Items {
		totalCents += item.UnitPriceCents * int64(item.Quantity)
	}

	now := time.Now()
	newOrder := &order.Order{
		ID:            "ord_" + uuid.New().String()[:8],
		CustomerID:    req.CustomerID,
		CustomerEmail: req.CustomerEmail,
		Items:         req.Items,
		TotalCents:    totalCents,
		Currency:      req.Currency,
		Status:        order.OrderStatus_ORDER_STATUS_PENDING,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	s.orders[newOrder.ID] = newOrder
	s.mu.Unlock()

	paymentResp, err := s.paymentClient.ProcessPayment(ctx, &payment.PaymentRequest{
		IdempotencyKey: newOrder.ID,
		OrderID:        newOrder.ID,
		AmountCents:    totalCents,
		Currency:       req.Currency,
		CustomerEmail:  req.CustomerEmail,
	})

	if err != nil {
		log.Printf("[ORDER] gRPC error calling Payment service: %v", err)
		s.updateOrderStatus(newOrder.ID, order.OrderStatus_ORDER_STATUS_CANCELLED)
		return nil, ErrPaymentServiceUnavailable
	}

	if !paymentResp.Success {
		s.updateOrderStatus(newOrder.ID, order.OrderStatus_ORDER_STATUS_CANCELLED)
		return nil, &PaymentDeclinedError{
			Code:    paymentResp.ErrorCode.String(),
			Message: paymentResp.ErrorMessage,
		}
	}

	s.mu.Lock()
	newOrder.Status = order.OrderStatus_ORDER_STATUS_PAID
	newOrder.PaymentTransactionID = paymentResp.TransactionID
	newOrder.UpdatedAt = time.Now()
	s.mu.Unlock()

	go s.publishOrderCreated(newOrder)

	return newOrder, nil
}

func (s *OrderService) publishOrderCreated(o *order.Order) {
	event := order.NewOrderCreatedEvent(*o)

	msg, err := broker.NewMessage("order.created", event)
	if err != nil {
		return
	}

	msg.SetMetadata("order_id", o.ID)
	msg.SetMetadata("customer_email", o.CustomerEmail)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.broker.Publish(ctx, s.topicName, msg)
}

func (s *OrderService) updateOrderStatus(orderID string, status order.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if o, ok := s.orders[orderID]; ok {
		o.Status = status
		o.UpdatedAt = time.Now()
	}
}

func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*order.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	o, ok := s.orders[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}

	return o, nil
}

func (s *OrderService) ListOrders(ctx context.Context) ([]*order.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]*order.Order, 0, len(s.orders))
	for _, o := range s.orders {
		orders = append(orders, o)
	}

	return orders, nil
}

func (s *OrderService) Stats() OrderStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := OrderStats{
		TotalOrders: len(s.orders),
	}

	for _, o := range s.orders {
		switch o.Status {
		case order.OrderStatus_ORDER_STATUS_PAID:
			stats.PaidOrders++
			stats.TotalRevenueCents += o.TotalCents
		case order.OrderStatus_ORDER_STATUS_CANCELLED:
			stats.CancelledOrders++
		case order.OrderStatus_ORDER_STATUS_PENDING:
			stats.PendingOrders++
		}
	}

	return stats
}

type OrderStats struct {
	TotalOrders       int
	PaidOrders        int
	CancelledOrders   int
	PendingOrders     int
	TotalRevenueCents int64
}
