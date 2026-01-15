package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/pkg/broker"
	_ "github.com/Tiago-De-Liz/go-microservices-grpc-messaging/pkg/codec"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/payment"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/order/internal/handler"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/order/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	httpPort := flag.Int("http-port", 8080, "HTTP server port")
	paymentAddr := flag.String("payment-addr", "localhost:50051", "Payment service gRPC address")
	flag.Parse()

	log.SetPrefix("[ORDER] ")
	log.Printf("Starting Order Service on port %d", *httpPort)
	log.Printf("Payment service at %s", *paymentAddr)

	paymentConn, err := grpc.NewClient(
		*paymentAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Payment service: %v", err)
	}
	defer paymentConn.Close()

	paymentClient := payment.NewPaymentServiceClient(paymentConn)
	log.Println("Connected to Payment service")

	msgBroker := broker.NewBroker(broker.DefaultBrokerConfig())
	msgBroker.CreateTopic("order.created")

	notificationQueue := msgBroker.CreateQueue("notifications", broker.WithMaxRetries(3))
	auditQueue := msgBroker.CreateQueue("audit", broker.WithMaxRetries(5))

	msgBroker.Subscribe("order.created", "notifications")
	msgBroker.Subscribe("order.created", "audit")
	log.Println("Message broker configured")

	go startNotificationWorker(notificationQueue)
	go startAuditWorker(auditQueue)

	orderSvc := service.NewOrderService(paymentClient, msgBroker, "order.created")
	orderHandler := handler.NewOrderHandler(orderSvc)

	mux := http.NewServeMux()
	orderHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *httpPort),
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("Order Service ready at http://localhost:%d", *httpPort)
	log.Println("Endpoints: POST /orders, GET /orders, GET /orders/{id}, GET /health, GET /stats")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func startNotificationWorker(queue *broker.Queue) {
	log.Println("[WORKER] Starting notification worker")

	worker := broker.NewWorker("notification-worker", queue, func(msg *broker.Message) error {
		var event struct {
			Order struct {
				ID            string `json:"id"`
				CustomerEmail string `json:"customer_email"`
				TotalCents    int64  `json:"total_cents"`
			} `json:"order"`
		}
		if err := msg.Decode(&event); err != nil {
			return err
		}

		log.Printf("[NOTIFICATION] 📧 Email to %s for order %s (R$ %.2f)",
			event.Order.CustomerEmail, event.Order.ID, float64(event.Order.TotalCents)/100)
		log.Printf("[NOTIFICATION] 📱 SMS for order %s", event.Order.ID)

		return nil
	})

	worker.Start(context.Background())
}

func startAuditWorker(queue *broker.Queue) {
	log.Println("[WORKER] Starting audit worker")

	worker := broker.NewWorker("audit-worker", queue, func(msg *broker.Message) error {
		var event struct {
			EventType string `json:"event_type"`
			Order     struct {
				ID         string `json:"id"`
				TotalCents int64  `json:"total_cents"`
				Status     int    `json:"status"`
			} `json:"order"`
		}
		if err := msg.Decode(&event); err != nil {
			return err
		}

		log.Printf("[AUDIT] 📝 %s | Order: %s | R$ %.2f | Status: %d",
			event.EventType, event.Order.ID, float64(event.Order.TotalCents)/100, event.Order.Status)

		return nil
	})

	worker.Start(context.Background())
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}
