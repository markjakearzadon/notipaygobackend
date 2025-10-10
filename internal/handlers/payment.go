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
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

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
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}
	userID, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, `{"error":"Invalid user_id in token"}`, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	paymentID := vars["paymentID"]
	if paymentID == "" {
		http.Error(w, `{"error":"Payment ID is required"}`, http.StatusBadRequest)
		return
	}

	payment, err := h.service.GetPaymentByID(r.Context(), paymentID)
	if err != nil {
		log.Printf("Failed to get payment %s: %v", paymentID, err)
		if strings.Contains(err.Error(), "payment not found") {
			http.Error(w, `{"error":"Payment not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payment: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if payment.PayerID != userID && payment.PayeeID != userID {
		http.Error(w, `{"error":"Unauthorized to view this payment"}`, http.StatusForbidden)
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
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

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
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}
	userID, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, `{"error":"Invalid user_id in token"}`, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	paymentID := vars["paymentID"]
	if paymentID == "" {
		http.Error(w, `{"error":"Payment ID is required"}`, http.StatusBadRequest)
		return
	}

	updatedPayment, err := h.service.UpdatePayment(r.Context(), paymentID, userID)
	if err != nil {
		log.Printf("Failed to update payment %s: %v", paymentID, err)
		if strings.Contains(err.Error(), "payment not found") {
			http.Error(w, `{"error":"Payment not found"}`, http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "can only update payment with status PENDING") {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"Failed to update payment. Please try again later."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Payment updated successfully",
		"payment": updatedPayment,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PayerCellphoneNumber string  `json:"payer_cellphone_number"`
		PayeeID              string  `json:"payee_id"`
		Amount               float64 `json:"amount"`
		Title                string  `json:"title"`
		Description          string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate phone number
	if req.PayerCellphoneNumber == "" {
		http.Error(w, `{"error":"Payer cellphone number is required"}`, http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.PayerCellphoneNumber, "0") || len(req.PayerCellphoneNumber) != 11 {
		http.Error(w, `{"error":"Payer cellphone number must start with 0 and be 11 digits"}`, http.StatusBadRequest)
		return
	}

	// Fetch user by phone number
	payer, err := h.service.GetUserByPhoneNumber(r.Context(), req.PayerCellphoneNumber)
	if err != nil {
		log.Printf("Failed to fetch user by cellphone number %s: %v", req.PayerCellphoneNumber, err)
		if strings.Contains(err.Error(), "user not found") {
			http.Error(w, `{"error":"User not found for provided cellphone number"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Use hardcoded payeeID as per your instruction
	req.PayeeID = "68e7c6efa33ffd411aebee85"

	// Fetch payee to get name
	payee, err := h.service.GetUserByID(r.Context(), req.PayeeID)
	if err != nil {
		log.Printf("Failed to fetch payee %s: %v", req.PayeeID, err)
		if strings.Contains(err.Error(), "user not found") {
			http.Error(w, `{"error":"Payee not found"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payee: %v"}`, err), http.StatusInternalServerError)
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

	// Combine names for payer and payee
	payerName := payer.FirstName
	if payer.MiddleInitial != "" {
		payerName += " " + payer.MiddleInitial
	}
	payerName += " " + payer.LastName
	payeeName := payee.FirstName
	if payee.MiddleInitial != "" {
		payeeName += " " + payee.MiddleInitial
	}
	payeeName += " " + payee.LastName

	payment, err := h.service.CreatePayment(r.Context(), payer.ID.Hex(), req.PayeeID, req.Amount, req.Title, req.Description, payerName, payeeName)
	if err != nil {
		log.Printf("Failed to create payment: %v", err)
		if strings.Contains(err.Error(), "payer not found") || strings.Contains(err.Error(), "payee not found") ||
			strings.Contains(err.Error(), "invalid payer_id") || strings.Contains(err.Error(), "invalid payee_id") ||
			strings.Contains(err.Error(), "payer email required") || strings.Contains(err.Error(), "payee CellphoneNumber missing") ||
			strings.Contains(err.Error(), "payee CellphoneNumber must start with 0") {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
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

func (h *PaymentHandler) CreateBulkPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount      float64 `json:"amount"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		ExcludeID   string  `json:"exclude_id"` // ID of the user to exclude (e.g., "trump")
		UserID      string  `json:"user_id"`    // Add user_id to request body
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
	if req.ExcludeID == "" {
		http.Error(w, `{"error":"Exclude ID is required"}`, http.StatusBadRequest)
		return
	}

	payments, err := h.service.CreateBulkPayment(r.Context(), "68e7c6efa33ffd411aebee85", req.ExcludeID, req.Amount, req.Title, req.Description)
	if err != nil {
		log.Printf("Failed to create bulk payment: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create bulk payment: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(payments); err != nil {
		log.Printf("Failed to encode payments: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Webhook processed successfully"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *PaymentHandler) GetPayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Only admins can view all payments
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
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}
	if claims["role"] != "admin" {
		http.Error(w, `{"error":"Admin access required"}`, http.StatusForbidden)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if statusFilter != "" && !map[string]bool{"PENDING": true, "SUCCEEDED": true, "EXPIRED": true}[statusFilter] {
		http.Error(w, `{"error":"Invalid status filter, must be PENDING, SUCCEEDED, or EXPIRED"}`, http.StatusBadRequest)
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

	payments, err := h.service.GetPayments(r.Context(), statusPtr, startDatePtr, endDatePtr)
	if err != nil {
		log.Printf("Failed to fetch payments: %v", err)
		if strings.Contains(err.Error(), "invalid status filter") || strings.Contains(err.Error(), "invalid start_date") || strings.Contains(err.Error(), "invalid end_date") {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
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
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}
	authenticatedUserID, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, `{"error":"Invalid user_id in token"}`, http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	requestedUserID := vars["userID"]
	if requestedUserID == "" {
		http.Error(w, `{"error":"User ID is required"}`, http.StatusBadRequest)
		return
	}

	if authenticatedUserID != requestedUserID && claims["role"] != "admin" {
		http.Error(w, `{"error":"Unauthorized to view payments for this user"}`, http.StatusForbidden)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if statusFilter != "" && !map[string]bool{"PENDING": true, "SUCCEEDED": true, "EXPIRED": true}[statusFilter] {
		http.Error(w, `{"error":"Invalid status filter, must be PENDING, SUCCEEDED, or EXPIRED"}`, http.StatusBadRequest)
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

	payments, err := h.service.GetPaymentsByUserID(r.Context(), requestedUserID, statusPtr, startDatePtr, endDatePtr)
	if err != nil {
		log.Printf("Failed to fetch payments for user %s: %v", requestedUserID, err)
		if strings.Contains(err.Error(), "invalid user_id") {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch payments: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payments); err != nil {
		log.Printf("Failed to encode payments: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}
