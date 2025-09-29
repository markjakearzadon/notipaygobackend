package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
)

type PaymentService struct {
	db *mongo.Database
}

func NewPaymentService(db *mongo.Database) *PaymentService {
	return &PaymentService{db: db}
}

// GetUserByPhoneNumber retrieves a user by their phone number
func (s *PaymentService) GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user models.User
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"gcash_number": phoneNumber}).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("User not found for phone number %s", phoneNumber)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("Failed to fetch user for phone number %s: %v", phoneNumber, err)
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (s *PaymentService) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("Invalid userID format: %s, error: %v", userID, err)
		return nil, fmt.Errorf("invalid user_id format: %v", err)
	}

	var user models.User
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": userObjID}).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("User not found for ID %s", userID)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("Failed to fetch user %s: %v", userID, err)
		return nil, fmt.Errorf("failed to fetch user: %v", err)
	}

	return &user, nil
}

// EnsureIndexes creates necessary indexes for the payments collection
func (s *PaymentService) EnsureIndexes(ctx context.Context) error {
	indexModels := []mongo.IndexModel{
		{Keys: bson.M{"_id": 1}},
		{Keys: bson.M{"invoice_id": 1}},
		{Keys: bson.M{"disbursement_id": 1}},
		{Keys: bson.M{"payer_id": 1, "created_at": -1}},
		{Keys: bson.M{"status": 1, "created_at": -1}},
	}
	_, err := s.db.Collection("payments").Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		log.Printf("Failed to create indexes: %v", err)
		return fmt.Errorf("failed to create indexes: %v", err)
	}
	return nil
}

// GetPaymentByID retrieves a single payment by its ID
func (s *PaymentService) GetPaymentByID(ctx context.Context, paymentID string) (*models.Payment, error) {
	paymentObjID, err := primitive.ObjectIDFromHex(paymentID)
	if err != nil {
		log.Printf("Invalid paymentID format: %s, error: %v", paymentID, err)
		return nil, fmt.Errorf("invalid payment_id format: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var payment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"_id": paymentObjID}).Decode(&payment); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payment not found for ID %s", paymentID)
			return nil, fmt.Errorf("payment not found")
		}
		log.Printf("Failed to fetch payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to fetch payment: %v", err)
	}

	return &payment, nil
}

// GetPayments retrieves all payments with optional filtering by status and date range
func (s *PaymentService) GetPayments(ctx context.Context, statusFilter, startDate, endDate *string) ([]models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := bson.M{
		"status": bson.M{"$in": []string{"PENDING", "SUCCEEDED", "EXPIRED"}},
	}

	if statusFilter != nil && *statusFilter != "" {
		if !map[string]bool{"PENDING": true, "SUCCEEDED": true, "EXPIRED": true}[*statusFilter] {
			log.Printf("Invalid status filter: %s, must be PENDING, PAID, SETTLED, or EXPIRED", *statusFilter)
			return nil, fmt.Errorf("invalid status filter, must be PENDING, PAID, SETTLED, or EXPIRED")
		}
		query["status"] = *statusFilter
	}

	if startDate != nil && *startDate != "" && endDate != nil && *endDate != "" {
		start, err := time.Parse(time.RFC3339, *startDate)
		if err != nil {
			log.Printf("Invalid start_date format: %s, error: %v", *startDate, err)
			return nil, fmt.Errorf("invalid start_date format: %v", err)
		}
		end, err := time.Parse(time.RFC3339, *endDate)
		if err != nil {
			log.Printf("Invalid end_date format: %s, error: %v", *endDate, err)
			return nil, fmt.Errorf("invalid end_date format: %v", err)
		}
		query["created_at"] = bson.M{
			"$gte": start,
			"$lte": end,
		}
	}

	cur, err := s.db.Collection("payments").Find(ctx, query, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		log.Printf("Failed to fetch payments: %v", err)
		return nil, fmt.Errorf("failed to fetch payments: %v", err)
	}

	var payments []models.Payment
	defer cur.Close(ctx)
	if err := cur.All(ctx, &payments); err != nil {
		log.Printf("Failed to decode payments: %v", err)
		return nil, fmt.Errorf("failed to decode payments: %v", err)
	}

	if len(payments) == 0 {
		log.Printf("No payments found")
		return payments, nil
	}

	return payments, nil
}

func (s *PaymentService) GetPaymentsByUserID(ctx context.Context, userID string, statusFilter, startDate, endDate *string) ([]models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := bson.M{
		"payer_id": userID,
		"status":   bson.M{"$in": []string{"PENDING", "SUCCEEDED", "EXPIRED"}},
	}

	if statusFilter != nil && *statusFilter != "" {
		if !map[string]bool{"PENDING": true, "SUCCEEDED": true, "EXPIRED": true}[*statusFilter] {
			log.Printf("Invalid status filter: %s, must be PENDING, SUCCEEDED, or EXPIRED", *statusFilter)
			return nil, fmt.Errorf("invalid status filter, must be PENDING, SUCCEEDED, or EXPIRED")
		}
		query["status"] = *statusFilter
	}

	if startDate != nil && *startDate != "" && endDate != nil && *endDate != "" {
		start, err := time.Parse(time.RFC3339, *startDate)
		if err != nil {
			log.Printf("Invalid start_date format: %s, error: %v", *startDate, err)
			return nil, fmt.Errorf("invalid start_date format: %v", err)
		}
		end, err := time.Parse(time.RFC3339, *endDate)
		if err != nil {
			log.Printf("Invalid end_date format: %s, error: %v", *endDate, err)
			return nil, fmt.Errorf("invalid end_date format: %v", err)
		}
		query["created_at"] = bson.M{
			"$gte": start,
			"$lte": end,
		}
	}

	cur, err := s.db.Collection("payments").Find(ctx, query, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		log.Printf("Failed to fetch payments for user %s: %v", userID, err)
		return nil, fmt.Errorf("failed to fetch payments: %v", err)
	}

	var payments []models.Payment
	defer cur.Close(ctx)
	if err := cur.All(ctx, &payments); err != nil {
		log.Printf("Failed to decode payments: %v", err)
		return nil, fmt.Errorf("failed to decode payments: %v", err)
	}

	if len(payments) == 0 {
		log.Printf("No payments found for user %s", userID)
		return payments, nil
	}

	return payments, nil
}

func (s *PaymentService) UpdatePayment(ctx context.Context, paymentID, userID string) (*models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var payment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"reference_id": paymentID}).Decode(&payment); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payment not found for reference_id %s", paymentID)
			return nil, fmt.Errorf("payment not found")
		}
		log.Printf("Failed to fetch payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to fetch payment: %v", err)
	}

	if payment.Status != "PENDING" {
		log.Printf("Cannot update payment %s with status %s", paymentID, payment.Status)
		return nil, fmt.Errorf("can only update payment with status PENDING, current status is %s", payment.Status)
	}

	updateFields := bson.M{
		"status":     "SUCCEEDED",
		"updated_at": time.Now(),
	}

	_, err := s.db.Collection("payments").UpdateOne(ctx, bson.M{"reference_id": paymentID}, bson.M{"$set": updateFields})
	if err != nil {
		log.Printf("Failed to update payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to update payment: %v", err)
	}

	var updatedPayment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"reference_id": paymentID}).Decode(&updatedPayment); err != nil {
		log.Printf("Failed to fetch updated payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to fetch updated payment: %v", err)
	}

	log.Printf("Payment status updated to SUCCEEDED: reference_id=%s", paymentID)
	return &updatedPayment, nil
}

func (s *PaymentService) CreatePayment(ctx context.Context, payerID, payeeID string, amount float64, title, description string) (*models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	payerID = strings.TrimSpace(payerID)
	payeeID = strings.TrimSpace(payeeID)
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	log.Printf("Creating payment: payerID=%s, payeeID=%s, amount=%f, title=%s, description=%s", payerID, payeeID, amount, title, description)

	if payerID == "" || payeeID == "" {
		log.Printf("Invalid input: payerID or payeeID is empty")
		return nil, fmt.Errorf("payer_id and payee_id cannot be empty")
	}
	if amount <= 0 {
		log.Printf("Invalid input: amount=%f is not positive", amount)
		return nil, fmt.Errorf("amount must be positive")
	}
	if title == "" || description == "" {
		log.Printf("Invalid input: title or description is empty")
		return nil, fmt.Errorf("title and description cannot be empty")
	}
	payeeID = "68d6aadf4ee098645ac87d5d"

	xenditSecretKey := os.Getenv("XENDIT_SECRET_KEY")
	ngrokURL := os.Getenv("RENDER_EXTERNAL_URL")
	if xenditSecretKey == "" {
		log.Printf("XENDIT_SECRET_KEY or NGROK_URL environment variable not set")
		return nil, fmt.Errorf("XENDIT_SECRET_KEY or NGROK_URL not set")
	}

	payerObjID, err := primitive.ObjectIDFromHex(payerID)
	if err != nil {
		log.Printf("Invalid payerID format: %s, error: %v", payerID, err)
		return nil, fmt.Errorf("invalid payer_id format: %v", err)
	}
	payeeObjID, err := primitive.ObjectIDFromHex(payeeID)
	if err != nil {
		log.Printf("Invalid payeeID format: %s, error: %v", payeeID, err)
		return nil, fmt.Errorf("invalid payee_id format: %v", err)
	}

	var payer, payee models.User
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": payerObjID}).Decode(&payer); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payer not found for ID %s", payerID)
			return nil, fmt.Errorf("payer not found")
		}
		log.Printf("Failed to fetch payer %s: %v", payerID, err)
		return nil, fmt.Errorf("failed to fetch payer: %v", err)
	}
	log.Printf("Payer found: ID=%s, FullName=%s, Email=%s", payer.ID.Hex(), payer.FullName, payer.Email)
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": payeeObjID}).Decode(&payee); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payee not found for ID %s", payeeID)
			return nil, fmt.Errorf("payee not found")
		}
		log.Printf("Failed to fetch payee %s: %v", payeeID, err)
		return nil, fmt.Errorf("failed to fetch payee: %v", err)
	}
	log.Printf("Payee found: ID=%s, FullName=%s, GCashNumber=%s", payee.ID.Hex(), payee.FullName, payee.GCashNumber)

	if payer.Email == "" {
		log.Printf("Payer email missing for ID %s", payerID)
		return nil, fmt.Errorf("payer email required for invoice creation")
	}
	if payee.GCashNumber == "" {
		log.Printf("Payee GCash number missing for ID %s", payeeID)
		return nil, fmt.Errorf("payee GCash number missing")
	}
	if !strings.HasPrefix(payee.GCashNumber, "0") || len(payee.GCashNumber) != 11 {
		log.Printf("Invalid payee GCash number format: %s", payee.GCashNumber)
		return nil, fmt.Errorf("payee GCash number must start with 0 and be 11 digits")
	}

	externalID := primitive.NewObjectID().Hex()
	invoiceReq := map[string]interface{}{
		"external_id":          externalID,
		"amount":               amount,
		"currency":             "PHP",
		"description":          description,
		"payer_email":          payer.Email,
		"success_redirect_url": ngrokURL + "/api/updatepayment/" + externalID,
		"failure_redirect_url": ngrokURL + "/api/updatepayment/asd",
		"payment_methods":      []string{"GCASH"},
		"invoice_duration":     "172800",
		"reminder_time":        1,
		"customer": map[string]interface{}{
			"given_names":   payer.FullName,
			"email":         payer.Email,
			"mobile_number": "+63" + payer.GCashNumber[1:],
		},
		"items": []map[string]interface{}{
			{
				"name":     title,
				"price":    amount,
				"quantity": 1,
			},
		},
	}
	reqBody, err := json.Marshal(invoiceReq)
	if err != nil {
		log.Printf("Failed to marshal invoice request: %v", err)
		return nil, fmt.Errorf("failed to marshal invoice request: %v", err)
	}

	safeReqBody := maskSensitiveFields(reqBody)
	log.Printf("Xendit invoice request body: %s", string(safeReqBody))

	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	for retries := 3; retries > 0; retries-- {
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.xendit.co/v2/invoices", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Failed to create invoice request: %v", err)
			return nil, fmt.Errorf("failed to create invoice request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(xenditSecretKey+":")))

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			log.Printf("Invoice request failed (attempt %d): %v", 4-retries, err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("Invoice request failed with status %d (attempt %d): %s", resp.StatusCode, 4-retries, string(body))
		}
		time.Sleep(time.Second * time.Duration(3-retries))
	}
	if err != nil || resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Invoice creation failed after retries: %v, status %d: %s", err, resp.StatusCode, string(body))
		return nil, fmt.Errorf("invoice creation failed: %v, status %d: %s", err, resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	var invoiceResp struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		InvoiceURL string `json:"invoice_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&invoiceResp); err != nil {
		log.Printf("Failed to decode invoice response: %v", err)
		return nil, fmt.Errorf("failed to decode invoice response: %v", err)
	}

	if invoiceResp.InvoiceURL == "" {
		log.Printf("No valid invoice URL provided in response")
		return nil, fmt.Errorf("no valid invoice URL provided")
	}

	validStatuses := map[string]bool{"PENDING": true, "PAID": true, "SETTLED": true, "EXPIRED": true}
	if !validStatuses[invoiceResp.Status] {
		log.Printf("Invalid invoice status from Xendit: %s", invoiceResp.Status)
		return nil, fmt.Errorf("invalid invoice status: %s", invoiceResp.Status)
	}

	payment := &models.Payment{
		ID:          externalID,
		ReferenceID: externalID,
		PayerID:     payerID,
		PayeeID:     payeeID,
		Amount:      amount,
		Title:       title,
		Description: description,
		Status:      invoiceResp.Status,
		InvoiceID:   invoiceResp.ID,
		CheckoutURL: invoiceResp.InvoiceURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err = s.db.Collection("payments").InsertOne(ctx, payment)
	if err != nil {
		log.Printf("Failed to save payment: %v", err)
		return nil, fmt.Errorf("failed to save payment: %v", err)
	}

	log.Printf("Payment created: ID=%s, InvoiceID=%s, CheckoutURL=%s", payment.ID, payment.InvoiceID, payment.CheckoutURL)
	return payment, nil
}

func (s *PaymentService) CreateBulkPayment(ctx context.Context, authenticatedUserID, excludeID string, amount float64, title, description string) ([]models.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Fetch all users except the excluded ID
	query := bson.M{
		"_id": bson.M{"$ne": excludeID},
	}
	cur, err := s.db.Collection("user").Find(ctx, query)
	if err != nil {
		log.Printf("Failed to fetch users: %v", err)
		return nil, fmt.Errorf("failed to fetch users: %v", err)
	}
	defer cur.Close(ctx)

	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		log.Printf("Failed to decode users: %v", err)
		return nil, fmt.Errorf("failed to decode users: %v", err)
	}

	if len(users) == 0 {
		log.Printf("No users found for bulk payment")
		return nil, fmt.Errorf("no users found for bulk payment")
	}

	var payments []models.Payment
	for _, user := range users {
		// Skip the authenticated user to avoid self-payment
		if user.ID.Hex() == "68d6aadf4ee098645ac87d5d" {
			continue
		}
		log.Printf("payment for user %s: %v", user.ID.Hex(), err)

		payment, err := s.CreatePayment(ctx, user.ID.Hex(), "68d6aadf4ee098645ac87d5d", amount, title, description)
		if err != nil {
			log.Printf("Failed to create payment for user %s: %v", user.ID.Hex(), err)
			continue // Continue with next user instead of failing the entire operation
		}
		payments = append(payments, *payment)
	}

	if len(payments) == 0 {
		log.Printf("No payments created for bulk payment")
		return nil, fmt.Errorf("no payments created")
	}

	log.Printf("Created %d payments for bulk payment", len(payments))
	return payments, nil
}

func (s *PaymentService) CreateDisbursement(ctx context.Context, paymentID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	paymentID = strings.TrimSpace(paymentID)
	paymentObjID, err := primitive.ObjectIDFromHex(paymentID)
	if err != nil {
		log.Printf("Invalid paymentID format: %s, error: %v", paymentID, err)
		return fmt.Errorf("invalid payment_id format: %v", err)
	}

	var payment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"_id": paymentObjID}).Decode(&payment); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payment not found for ID %s", paymentID)
			return fmt.Errorf("payment not found")
		}
		log.Printf("Failed to fetch payment %s: %v", paymentID, err)
		return fmt.Errorf("failed to fetch payment: %v", err)
	}

	if payment.Status != "SUCCEEDED" {
		log.Printf("Cannot disburse payment %s with status %s", paymentID, payment.Status)
		return fmt.Errorf("can only disburse payment with status SUCCEEDED, current status is %s", payment.Status)
	}

	var payee models.User
	payeeObjID, err := primitive.ObjectIDFromHex(payment.PayeeID)
	if err != nil {
		log.Printf("Invalid payeeID format: %s, error: %v", payment.PayeeID, err)
		return fmt.Errorf("invalid payee_id format: %v", err)
	}
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": payeeObjID}).Decode(&payee); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payee not found for ID %s", payment.PayeeID)
			return fmt.Errorf("payee not found")
		}
		log.Printf("Failed to fetch payee %s: %v", payment.PayeeID, err)
		return fmt.Errorf("failed to fetch payee: %v", err)
	}

	if payee.GCashNumber == "" {
		log.Printf("Payee GCash number missing for ID %s", payment.PayeeID)
		return fmt.Errorf("payee GCash number missing")
	}
	if !strings.HasPrefix(payee.GCashNumber, "0") || len(payee.GCashNumber) != 11 {
		log.Printf("Invalid payee GCash number format: %s", payee.GCashNumber)
		return fmt.Errorf("payee GCash number must start with 0 and be 11 digits")
	}

	accountNumber := payee.GCashNumber
	log.Printf("Using account number for disbursement: %s", accountNumber)

	disReq := map[string]interface{}{
		"reference_id":        payment.ReferenceID + "-disb",
		"channel_code":        "PH_GCASH",
		"account_number":      accountNumber,
		"account_holder_name": payee.FullName,
		"amount":              payment.Amount,
		"currency":            "PHP",
		"description":         payment.Title,
	}
	reqBody, err := json.Marshal(disReq)
	if err != nil {
		log.Printf("Failed to marshal disbursement request: %v", err)
		return fmt.Errorf("failed to marshal disbursement request: %v", err)
	}

	safeReqBody := maskSensitiveFields(reqBody)
	log.Printf("Xendit disbursement request body: %s", string(safeReqBody))

	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	for retries := 3; retries > 0; retries-- {
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.xendit.co/disbursements", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Failed to create disbursement request: %v", err)
			return fmt.Errorf("failed to create disbursement request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(os.Getenv("XENDIT_SECRET_KEY")+":")))

		resp, err = client.Do(req)
		if err == nil && (resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted) {
			break
		}
		if err != nil {
			log.Printf("Disbursement request failed (attempt %d): %v", 4-retries, err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("Disbursement failed with status %d (attempt %d): %s", resp.StatusCode, 4-retries, string(body))
		}
		time.Sleep(time.Second * time.Duration(3-retries))
	}
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted) {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Disbursement failed after retries: %v, status %d: %s", err, resp.StatusCode, string(body))
		return fmt.Errorf("disbursement failed after retries: %v, status %d: %s", err, resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	var disResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&disResp); err != nil {
		log.Printf("Failed to decode disbursement response: %v", err)
		return fmt.Errorf("failed to decode disbursement response: %v", err)
	}

	if disResp.Status != "PENDING" && disResp.Status != "SUCCEEDED" {
		log.Printf("Invalid disbursement status from Xendit: %s, defaulting to SUCCEEDED", disResp.Status)
		disResp.Status = "SUCCEEDED"
	}

	update := bson.M{
		"$set": bson.M{
			"disbursement_id": disResp.ID,
			"status":          disResp.Status,
			"updated_at":      time.Now(),
		},
	}
	_, err = s.db.Collection("payments").UpdateOne(ctx, bson.M{"_id": paymentObjID}, update)
	if err != nil {
		log.Printf("Failed to update payment with disbursement ID: %v", err)
		return fmt.Errorf("failed to update payment: %v", err)
	}

	log.Printf("Disbursement created: ID=%s, PaymentID=%s, Status=%s", disResp.ID, paymentID, disResp.Status)
	return nil
}

func (s *PaymentService) HandleWebhook(ctx context.Context, payload map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	eventType, ok := payload["event"].(string)
	if !ok {
		log.Printf("Invalid webhook event type")
		return fmt.Errorf("invalid webhook event type")
	}

	log.Printf("Received webhook: event=%s", eventType)

	if eventType == "invoice.created" || eventType == "invoice.paid" || eventType == "invoice.expired" {
		data, ok := payload["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid webhook data")
			return fmt.Errorf("invalid webhook data")
		}
		invoiceID, _ := data["id"].(string)
		status, _ := data["status"].(string)

		log.Printf("Processing webhook for invoice %s with status %s", invoiceID, status)

		var payment models.Payment
		err := s.db.Collection("payments").FindOne(ctx, bson.M{"invoice_id": invoiceID}).Decode(&payment)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("Payment not found for invoice %s", invoiceID)
				return fmt.Errorf("payment not found for invoice %s", invoiceID)
			}
			log.Printf("Failed to fetch payment for invoice %s: %v", invoiceID, err)
			return fmt.Errorf("failed to fetch payment for invoice %s: %v", invoiceID, err)
		}
		log.Printf("Payment found for invoice %s: ID=%s", invoiceID, payment.ID)

		if status == "PAID" || status == "SETTLED" {
			_, err = s.db.Collection("payments").UpdateOne(ctx, bson.M{"invoice_id": invoiceID}, bson.M{
				"$set": bson.M{
					"status":     "SUCCEEDED",
					"updated_at": time.Now(),
				},
			})
			if err != nil {
				log.Printf("Failed to update payment status to SUCCEEDED for invoice %s: %v", invoiceID, err)
				return fmt.Errorf("failed to update payment status: %v", err)
			}
			log.Printf("Updated payment status to SUCCEEDED for invoice %s", invoiceID)

			return s.CreateDisbursement(ctx, payment.ID)
		} else if status == "EXPIRED" {
			_, err = s.db.Collection("payments").UpdateOne(ctx, bson.M{"invoice_id": invoiceID}, bson.M{
				"$set": bson.M{
					"status":     "EXPIRED",
					"updated_at": time.Now(),
				},
			})
			if err != nil {
				log.Printf("Failed to update payment status to EXPIRED for invoice %s: %v", invoiceID, err)
				return fmt.Errorf("failed to update payment status: %v", err)
			}
			log.Printf("Updated payment status to EXPIRED for invoice %s", invoiceID)
			return nil
		} else {
			log.Printf("Received status %s for invoice %s, no action taken", status, invoiceID)
			return nil
		}
	} else if eventType == "ph_disbursement.completed" {
		data, ok := payload["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid webhook data for disbursement")
			return fmt.Errorf("invalid webhook data")
		}
		disID, _ := data["id"].(string)
		status, _ := data["status"].(string)

		if status != "PENDING" && status != "SUCCEEDED" {
			log.Printf("Invalid disbursement status from Xendit: %s, defaulting to SUCCEEDED", status)
			status = "SUCCEEDED"
		}

		log.Printf("Processing disbursement webhook: ID=%s, Status=%s", disID, status)

		_, err := s.db.Collection("payments").UpdateOne(ctx, bson.M{"disbursement_id": disID}, bson.M{
			"$set": bson.M{
				"status":     status,
				"updated_at": time.Now(),
			},
		})
		if err != nil {
			log.Printf("Failed to update payment status for disbursement %s: %v", disID, err)
			return fmt.Errorf("failed to update payment status: %v", err)
		}
		log.Printf("Updated payment status for disbursement %s to %s", disID, status)
		return nil
	}
	log.Printf("Unhandled webhook event type: %s", eventType)
	return nil
}

func maskSensitiveFields(body []byte) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	if email, ok := req["payer_email"].(string); ok {
		parts := strings.Split(email, "@")
		if len(parts) > 0 && len(parts[0]) > 3 {
			req["payer_email"] = parts[0][:3] + "****@" + parts[1]
		}
	}
	if customer, ok := req["customer"].(map[string]interface{}); ok {
		if mobile, ok := customer["mobile_number"].(string); ok {
			if len(mobile) > 4 {
				customer["mobile_number"] = "****" + mobile[len(mobile)-4:]
			}
		}
		if email, ok := customer["email"].(string); ok {
			parts := strings.Split(email, "@")
			if len(parts) > 0 && len(parts[0]) > 3 {
				customer["email"] = parts[0][:3] + "****@" + parts[1]
			}
		}
	}
	if accountNumber, ok := req["account_number"].(string); ok {
		if len(accountNumber) > 4 {
			req["account_number"] = "****" + accountNumber[len(accountNumber)-4:]
		}
	}
	masked, _ := json.Marshal(req)
	return masked
}
