package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/proto/order"
	"github.com/Tiago-De-Liz/go-microservices-grpc-messaging/services/order/internal/service"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/orders", h.handleOrders)
	mux.HandleFunc("/orders/", h.handleOrderByID)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/stats", h.handleStats)
}

func (h *OrderHandler) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listOrders(w, r)
	case http.MethodPost:
		h.createOrder(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *OrderHandler) handleOrderByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}
	orderID := parts[2]

	switch r.Method {
	case http.MethodGet:
		h.getOrder(w, r, orderID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type CreateOrderRequest struct {
	CustomerID    string      `json:"customer_id"`
	CustomerEmail string      `json:"customer_email"`
	Items         []OrderItem `json:"items"`
	Currency      string      `json:"currency"`
}

type OrderItem struct {
	ProductID      string `json:"product_id"`
	ProductName    string `json:"product_name"`
	Quantity       int32  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

func (h *OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("[HTTP] POST /orders: customer=%s email=%s items=%d",
		req.CustomerID, req.CustomerEmail, len(req.Items))

	items := make([]order.OrderItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = order.OrderItem{
			ProductID:      item.ProductID,
			ProductName:    item.ProductName,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		}
	}

	currency := req.Currency
	if currency == "" {
		currency = "BRL"
	}

	result, err := h.svc.CreateOrder(r.Context(), service.CreateOrderRequest{
		CustomerID:    req.CustomerID,
		CustomerEmail: req.CustomerEmail,
		Items:         items,
		Currency:      currency,
	})

	if err != nil {
		log.Printf("[HTTP] POST /orders error: %v", err)

		switch {
		case err == service.ErrNoItems:
			respondError(w, http.StatusBadRequest, "At least one item is required")
		case err == service.ErrMissingEmail:
			respondError(w, http.StatusBadRequest, "Customer email is required")
		case err == service.ErrPaymentServiceUnavailable:
			respondError(w, http.StatusServiceUnavailable, "Payment service unavailable")
		case service.IsPaymentDeclined(err):
			respondError(w, http.StatusPaymentRequired, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "Internal error")
		}
		return
	}

	log.Printf("[HTTP] POST /orders success: order=%s status=%s", result.ID, result.Status)
	respondJSON(w, http.StatusCreated, result)
}

func (h *OrderHandler) listOrders(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] GET /orders")

	orders, err := h.svc.ListOrders(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list orders")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"orders": orders,
		"count":  len(orders),
	})
}

func (h *OrderHandler) getOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	log.Printf("[HTTP] GET /orders/%s", orderID)

	o, err := h.svc.GetOrder(r.Context(), orderID)
	if err != nil {
		if err == service.ErrOrderNotFound {
			respondError(w, http.StatusNotFound, "Order not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to get order")
		return
	}

	respondJSON(w, http.StatusOK, o)
}

func (h *OrderHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "order",
	})
}

func (h *OrderHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.svc.Stats()
	respondJSON(w, http.StatusOK, stats)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
