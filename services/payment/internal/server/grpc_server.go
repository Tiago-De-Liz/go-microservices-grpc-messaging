package server

import (
	"context"
	"log"

	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/payment"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/payment/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentServer struct {
	payment.UnimplementedPaymentServiceServer
	svc *service.PaymentService
}

func NewPaymentServer(svc *service.PaymentService) *PaymentServer {
	return &PaymentServer{svc: svc}
}

func (s *PaymentServer) ProcessPayment(ctx context.Context, req *payment.PaymentRequest) (*payment.PaymentResponse, error) {
	log.Printf("[GRPC] ProcessPayment: order=%s amount=%d currency=%s",
		req.OrderID, req.AmountCents, req.Currency)

	if err := validatePaymentRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resp, err := s.svc.ProcessPayment(ctx, req)
	if err != nil {
		log.Printf("[GRPC] ProcessPayment error: %v", err)
		return nil, status.Error(codes.Internal, "payment processing failed")
	}

	if resp.Success {
		log.Printf("[GRPC] ProcessPayment success: transaction=%s", resp.TransactionID)
	} else {
		log.Printf("[GRPC] ProcessPayment declined: code=%s", resp.ErrorCode)
	}

	return resp, nil
}

func (s *PaymentServer) GetPaymentStatus(ctx context.Context, req *payment.PaymentStatusRequest) (*payment.PaymentStatusResponse, error) {
	log.Printf("[GRPC] GetPaymentStatus: transaction=%s", req.TransactionID)

	if req.TransactionID == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	resp, err := s.svc.GetPaymentStatus(ctx, req)
	if err != nil {
		if err == service.ErrTransactionNotFound {
			return nil, status.Error(codes.NotFound, "transaction not found")
		}
		return nil, status.Error(codes.Internal, "failed to get status")
	}

	return resp, nil
}

func validatePaymentRequest(req *payment.PaymentRequest) error {
	if req.OrderID == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}
	if req.AmountCents <= 0 {
		return status.Error(codes.InvalidArgument, "amount must be positive")
	}
	if req.Currency == "" {
		return status.Error(codes.InvalidArgument, "currency is required")
	}
	if req.IdempotencyKey == "" {
		return status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	return nil
}
