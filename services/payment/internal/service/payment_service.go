package service

import (
	"context"
	"sync"
	"time"

	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/payment"
	"github.com/google/uuid"
)

type PaymentService struct {
	mu            sync.RWMutex
	transactions  map[string]*payment.PaymentStatusResponse
	processedKeys map[string]*payment.PaymentResponse
	config        PaymentConfig
}

type PaymentConfig struct {
	MaxAmountCents  int64
	SimulateLatency time.Duration
	FailureRate     float64
}

func DefaultPaymentConfig() PaymentConfig {
	return PaymentConfig{
		MaxAmountCents:  1000000,
		SimulateLatency: 100 * time.Millisecond,
		FailureRate:     0.0,
	}
}

func NewPaymentService(config PaymentConfig) *PaymentService {
	return &PaymentService{
		transactions:  make(map[string]*payment.PaymentStatusResponse),
		processedKeys: make(map[string]*payment.PaymentResponse),
		config:        config,
	}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, req *payment.PaymentRequest) (*payment.PaymentResponse, error) {
	if s.config.SimulateLatency > 0 {
		time.Sleep(s.config.SimulateLatency)
	}

	s.mu.RLock()
	if cached, ok := s.processedKeys[req.IdempotencyKey]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	response := s.processPaymentInternal(req)

	s.mu.Lock()
	s.processedKeys[req.IdempotencyKey] = response
	if response.Success {
		s.transactions[response.TransactionID] = &payment.PaymentStatusResponse{
			TransactionID: response.TransactionID,
			OrderID:       req.OrderID,
			AmountCents:   req.AmountCents,
			Currency:      req.Currency,
			Status:        payment.PaymentStatus_PAYMENT_STATUS_COMPLETED,
			CreatedAt:     response.ProcessedAt,
		}
	}
	s.mu.Unlock()

	return response, nil
}

func (s *PaymentService) processPaymentInternal(req *payment.PaymentRequest) *payment.PaymentResponse {
	now := time.Now()

	if req.AmountCents <= 0 {
		return &payment.PaymentResponse{
			Success:      false,
			ErrorCode:    payment.PaymentErrorCode_PAYMENT_ERROR_CODE_PROCESSING_ERROR,
			ErrorMessage: "Amount must be positive",
			ProcessedAt:  now,
		}
	}

	if req.AmountCents > s.config.MaxAmountCents {
		return &payment.PaymentResponse{
			Success:      false,
			ErrorCode:    payment.PaymentErrorCode_PAYMENT_ERROR_CODE_LIMIT_EXCEEDED,
			ErrorMessage: "Amount exceeds maximum allowed",
			ProcessedAt:  now,
		}
	}

	if req.OrderID == "" {
		return &payment.PaymentResponse{
			Success:      false,
			ErrorCode:    payment.PaymentErrorCode_PAYMENT_ERROR_CODE_PROCESSING_ERROR,
			ErrorMessage: "Order ID is required",
			ProcessedAt:  now,
		}
	}

	if req.AmountCents%100 == 99 {
		return &payment.PaymentResponse{
			Success:      false,
			ErrorCode:    payment.PaymentErrorCode_PAYMENT_ERROR_CODE_INVALID_CARD,
			ErrorMessage: "Card declined (simulated)",
			ProcessedAt:  now,
		}
	}

	transactionID := "tx_" + uuid.New().String()[:8]

	return &payment.PaymentResponse{
		Success:       true,
		TransactionID: transactionID,
		ProcessedAt:   now,
	}
}

func (s *PaymentService) GetPaymentStatus(ctx context.Context, req *payment.PaymentStatusRequest) (*payment.PaymentStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[req.TransactionID]
	if !ok {
		return nil, ErrTransactionNotFound
	}

	return tx, nil
}

func (s *PaymentService) Stats() PaymentStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := PaymentStats{
		TotalTransactions:   len(s.transactions),
		CachedIdempotencies: len(s.processedKeys),
	}

	var totalAmount int64
	for _, tx := range s.transactions {
		totalAmount += tx.AmountCents
	}
	stats.TotalAmountCents = totalAmount

	return stats
}

type PaymentStats struct {
	TotalTransactions   int
	TotalAmountCents    int64
	CachedIdempotencies int
}
