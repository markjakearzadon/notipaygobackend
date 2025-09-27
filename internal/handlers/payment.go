package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
)

type PaymentHandler struct {
	service *services.PaymentService
}

func NewPaymentHandler(service *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) GetPaymentHandler(w http.ResponseWriter, r *http.Request) {
	// Verify JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	// Extract payment ID from URL
	vars := mux.Vars(r)
	paymentID := vars["paymentID"]
	if paymentID == "" {
		http.Error(w, `{"error":"Payment ID is required"}`, http.StatusBadRequest)
		return
	}

	// Fetch payment
	payment, err := h.service.GetPaymentByID(r.Context(), paymentID)
	if err != nil {
		log.Printf("Failed to get payment %s: %v", paymentID, err)
		if strings.Contains(err.Error(), "payment not found") {
			http.Error(w, `{"error":"payment not found"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payment: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(payment); err != nil {
		log.Printf("Failed to encode payment: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) UpdatePayment(w http.ResponseWriter, r *http.Request) {
	// Verify JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	// Extract payment ID from URL
	vars := mux.Vars(r)
	paymentID := vars["paymentID"]
	if paymentID == "" {
		http.Error(w, `{"error":"Payment ID is required"}`, http.StatusBadRequest)
		return
	}

	// Call service to update payment status to SUCCEEDED
	updatedPayment, err := h.service.UpdatePayment(r.Context(), paymentID, "")
	if err != nil {
		log.Printf("Failed to update payment %s: %v", paymentID, err)
		if strings.Contains(err.Error(), "payment not found") {
			http.Error(w, `{"error":"payment not found"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to update payment: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedPayment); err != nil {
		log.Printf("Failed to encode updated payment: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	// Verify JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		PayerNumber string  `json:"payer_number"` // Changed from payer_id
		PayeeNumber string  `json:"payee_number"` // Changed from payee_id
		Amount      float64 `json:"amount"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, `{"error":"Amount must be positive"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"Title is required"}`, http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		http.Error(w, `{"error":"Description is required"}`, http.StatusBadRequest)
		return
	}

	payment, err := h.service.CreatePayment(r.Context(), req.PayerNumber, req.PayeeNumber, req.Amount, req.Title, req.Description)
	if err != nil {
		log.Printf("Failed to create payment: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create payment: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(payment); err != nil {
		log.Printf("Failed to encode payment: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("x-callback-token")
	if token != os.Getenv("XENDIT_WEBHOOK_TOKEN") {
		http.Error(w, `{"error":"Unauthorized webhook"}`, http.StatusUnauthorized)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"Invalid webhook payload"}`, http.StatusBadRequest)
		return
	}

	if err := h.service.HandleWebhook(r.Context(), payload); err != nil {
		log.Printf("Webhook processing failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Webhook processing failed: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PaymentHandler) GetPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters for filtering
	statusFilter := r.URL.Query().Get("status")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// Validate status filter
	if statusFilter != "" && statusFilter != "PENDING" && statusFilter != "SUCCEEDED" {
		http.Error(w, `{"error":"Invalid status filter, must be PENDING or SUCCEEDED"}`, http.StatusBadRequest)
		return
	}

	var statusPtr, startDatePtr, endDatePtr *string
	if statusFilter != "" {
		statusPtr = &statusFilter
	}
	if startDate != "" {
		startDatePtr = &startDate
	}
	if endDate != "" {
		endDatePtr = &endDate
	}

	// Fetch all payments
	payments, err := h.service.GetPayments(r.Context(), statusPtr, startDatePtr, endDatePtr)
	if err != nil {
		log.Printf("Failed to fetch payments: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payments: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payments); err != nil {
		log.Printf("Failed to encode payments: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) GetPaymentsByUserID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Verify JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"Authorization header required"}`, http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}
	authenticatedUserNumber, ok := claims["gcash_number"].(string)
	if !ok {
		http.Error(w, `{"error":"Invalid gcash_number in token"}`, http.StatusUnauthorized)
		return
	}

	// Extract user number from URL
	vars := mux.Vars(r)
	requestedUserNumber := vars["userNumber"]
	if requestedUserNumber == "" {
		http.Error(w, `{"error":"User number is required"}`, http.StatusBadRequest)
		return
	}

	// Check if the authenticated user is requesting their own payments
	if authenticatedUserNumber != requestedUserNumber {
		http.Error(w, `{"error":"Unauthorized to view payments for this user"}`, http.StatusForbidden)
		return
	}

	// Parse query parameters for filtering
	statusFilter := r.URL.Query().Get("status")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// Validate status filter
	if statusFilter != "" && statusFilter != "PENDING" && statusFilter != "SUCCEEDED" {
		http.Error(w, `{"error":"Invalid status filter, must be PENDING or SUCCEEDED"}`, http.StatusBadRequest)
		return
	}

	var statusPtr, startDatePtr, endDatePtr *string
	if statusFilter != "" {
		statusPtr = &statusFilter
	}
	if startDate != "" {
		startDatePtr = &startDate
	}
	if endDate != "" {
		endDatePtr = &endDate
	}

	// Fetch payments for the requested user
	payments, err := h.service.GetPaymentsByUserNumber(r.Context(), requestedUserNumber, statusPtr, startDatePtr, endDatePtr)
	if err != nil {
		log.Printf("Failed to fetch payments for user %s: %v", requestedUserNumber, err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payments: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payments); err != nil {
		log.Printf("Failed to encode payments: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}
