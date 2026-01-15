package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/Tiago-De-Liz/go-microservices-grpc-messaging/pkg/codec"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/payment"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/payment/internal/server"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/payment/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := flag.Int("port", 50051, "gRPC server port")
	flag.Parse()

	log.SetPrefix("[PAYMENT] ")
	log.Printf("Starting Payment Service on port %d", *port)

	paymentSvc := service.NewPaymentService(service.DefaultPaymentConfig())
	paymentServer := server.NewPaymentServer(paymentSvc)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	payment.RegisterPaymentServiceServer(grpcServer, paymentServer)
	reflection.Register(grpcServer)

	addr := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Payment Service ready at %s", addr)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	log.Printf("→ %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		log.Printf("← %s ERROR: %v", info.FullMethod, err)
	} else {
		log.Printf("← %s OK", info.FullMethod)
	}
	return resp, err
}
