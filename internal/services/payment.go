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

// GetPaymentByID retrieves a single payment by its ID.
func (s *PaymentService) GetPaymentByID(ctx context.Context, paymentID string) (*models.Payment, error) {
	// Validate paymentID format
	paymentObjID, err := primitive.ObjectIDFromHex(paymentID)
	if err != nil {
		log.Printf("Invalid paymentID format: %s, error: %v", paymentID, err)
		return nil, fmt.Errorf("invalid payment_id format: %v", err)
	}

	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Find the payment
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

// GetPayments retrieves all payments with optional filtering by status and date range.
func (s *PaymentService) GetPayments(ctx context.Context, statusFilter, startDate, endDate *string) ([]models.Payment, error) {
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Build query
	query := bson.M{
		"status": bson.M{"$in": []string{"PENDING", "SUCCEEDED"}}, // Only include PENDING and SUCCEEDED
	}

	// Add status filter if provided
	if statusFilter != nil && *statusFilter != "" {
		if *statusFilter != "PENDING" && *statusFilter != "SUCCEEDED" {
			log.Printf("Invalid status filter: %s, must be PENDING or SUCCEEDED", *statusFilter)
			return nil, fmt.Errorf("invalid status filter, must be PENDING or SUCCEEDED")
		}
		query["status"] = *statusFilter
	}

	// Add date range filter if provided
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

	// Execute query
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

func (s *PaymentService) UpdatePayment(ctx context.Context, paymentID, userID string) (*models.Payment, error) {
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Validate paymentID format
	paymentObjID := paymentID

	// Find the payment
	var payment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"_id": paymentObjID}).Decode(&payment); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payment not found for ID %s", paymentID)
			return nil, fmt.Errorf("payment not found")
		}
		log.Printf("Failed to fetch payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to fetch payment: %v", err)
	}

	// Only allow updates if payment status is PENDING
	if payment.Status != "PENDING" {
		log.Printf("Cannot update payment %s with status %s", paymentID, payment.Status)
		return nil, fmt.Errorf("can only update payment with status PENDING, current status is %s", payment.Status)
	}

	// Update payment status to SUCCEEDED
	updateFields := bson.M{
		"status":     "SUCCEEDED",
		"updated_at": time.Now(),
	}

	// Update payment in database
	_, err := s.db.Collection("payments").UpdateOne(ctx, bson.M{"_id": paymentObjID}, bson.M{"$set": updateFields})
	if err != nil {
		log.Printf("Failed to update payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to update payment: %v", err)
	}

	// Fetch updated payment
	var updatedPayment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"_id": paymentObjID}).Decode(&updatedPayment); err != nil {
		log.Printf("Failed to fetch updated payment %s: %v", paymentID, err)
		return nil, fmt.Errorf("failed to fetch updated payment: %v", err)
	}

	log.Printf("Payment status updated to SUCCEEDED: ID=%s", paymentID)
	return &updatedPayment, nil
}

func (s *PaymentService) CreatePayment(ctx context.Context, payerID, payeeID string, amount float64, title, description string) (*models.Payment, error) {
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Log input
	payerID = strings.TrimSpace(payerID)
	payeeID = strings.TrimSpace(payeeID)
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	log.Printf("Creating payment: payerID=%s, payeeID=%s, amount=%f, title=%s, description=%s", payerID, payeeID, amount, title, description)

	// Validate input
	if payerID == "" || payeeID == "" {
		log.Printf("Invalid input: payerID or payeeID is empty")
		return nil, fmt.Errorf("payer_id and payee_id cannot be empty")
	}
	if amount <= 0 {
		log.Printf("Invalid input: amount=%f is not positive", amount)
		return nil, fmt.Errorf("amount must be positive")
	}
	if title == "" {
		log.Printf("Invalid input: title is empty")
		return nil, fmt.Errorf("title cannot be empty")
	}
	if description == "" {
		log.Printf("Invalid input: description is empty")
		return nil, fmt.Errorf("description cannot be empty")
	}

	// Convert string IDs to ObjectID
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

	// Validate payer and payee
	var payer, payee models.User
	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": payerObjID}).Decode(&payer); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payer not found for ID %s", payerID)
			return nil, fmt.Errorf("payer not found")
		}
		log.Printf("Failed to fetch payer %s: %v", payerID, err)
		return nil, fmt.Errorf("failed to fetch payer: %v", err)
	}
	log.Printf("Payer found: ID=%s, FullName=%s, GCashNumber=%s", payer.ID.Hex(), payer.FullName, payer.GCashNumber)

	if err := s.db.Collection("user").FindOne(ctx, bson.M{"_id": payeeObjID}).Decode(&payee); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payee not found for ID %s", payeeID)
			return nil, fmt.Errorf("payee not found")
		}
		log.Printf("Failed to fetch payee %s: %v", payeeID, err)
		return nil, fmt.Errorf("failed to fetch payee: %v", err)
	}
	log.Printf("Payee found: ID=%s, FullName=%s, GCashNumber=%s", payee.ID.Hex(), payee.FullName, payee.GCashNumber)

	if payer.GCashNumber == "" || payee.GCashNumber == "" {
		log.Printf("GCash number missing: payer=%s, payee=%s", payer.GCashNumber, payee.GCashNumber)
		return nil, fmt.Errorf("payer or payee GCash number missing")
	}

	// Validate GCash number format (must start with 0 and have 11 digits)
	if !strings.HasPrefix(payer.GCashNumber, "0") || len(payer.GCashNumber) != 11 {
		log.Printf("Invalid payer GCash number format: %s", payer.GCashNumber)
		return nil, fmt.Errorf("payer GCash number must start with 0 and be 11 digits")
	}

	// Format mobile number for Xendit (use +63 for eWallet charge)
	mobileNumber := "+63" + payer.GCashNumber[1:]
	log.Printf("Formatted mobile number for Xendit: %s", mobileNumber)

	// Get ngrok URL from environment variable
	ngrokURL := os.Getenv("NGROK_URL")
	if ngrokURL == "" {
		log.Printf("NGROK_URL environment variable not set")
		return nil, fmt.Errorf("NGROK_URL environment variable not set")
	}
	log.Printf("Using NGROK_URL: %s", ngrokURL)

	// Prepare Xendit charge request
	referenceID := primitive.NewObjectID().Hex()
	chargeReq := map[string]interface{}{
		"reference_id":    referenceID,
		"channel_code":    "PH_GCASH",
		"amount":          amount,
		"currency":        "PHP",
		"checkout_method": "ONE_TIME_PAYMENT",
		"title":           title,
		"description":     description,
		"channel_properties": map[string]interface{}{
			"mobile_number":        mobileNumber,
			"success_redirect_url": ngrokURL + "/success",
			"failure_redirect_url": ngrokURL + "/failure",
		},
	}
	reqBody, err := json.Marshal(chargeReq)
	if err != nil {
		log.Printf("Failed to marshal charge request: %v", err)
		return nil, fmt.Errorf("failed to marshal charge request: %v", err)
	}

	// Log the request body for debugging
	log.Printf("Xendit charge request body: %s", string(reqBody))

	// Make HTTP request to Xendit with retry logic
	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	for retries := 3; retries > 0; retries-- {
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.xendit.co/ewallets/charges", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Failed to create charge request: %v", err)
			return nil, fmt.Errorf("failed to create charge request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(os.Getenv("XENDIT_SECRET_KEY")+":")))

		resp, err = client.Do(req)
		if err == nil && (resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusAccepted) {
			break
		}
		if err != nil {
			log.Printf("Charge request failed (attempt %d): %v", 4-retries, err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("Charge failed with status %d (attempt %d): %s", resp.StatusCode, 4-retries, string(body))
		}
		time.Sleep(time.Second * time.Duration(3-retries))
	}
	if err != nil || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted) {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Charge failed after retries: %v, status %d: %s", err, resp.StatusCode, string(body))
		return nil, fmt.Errorf("charge failed after retries: %v, status %d: %s", err, resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	var chargeResp struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Actions struct {
			MobileDeeplinkCheckoutURL string `json:"mobile_deeplink_checkout_url"`
			MobileWebCheckoutURL      string `json:"mobile_web_checkout_url"`
			DesktopWebCheckoutURL     string `json:"desktop_web_checkout_url"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chargeResp); err != nil {
		log.Printf("Failed to decode charge response: %v", err)
		return nil, fmt.Errorf("failed to decode charge response: %v", err)
	}

	// Use mobile_deeplink_checkout_url if available, otherwise fall back to mobile_web_checkout_url
	checkoutURL := chargeResp.Actions.MobileDeeplinkCheckoutURL
	if checkoutURL == "" {
		checkoutURL = chargeResp.Actions.MobileWebCheckoutURL
		log.Printf("Mobile deeplink URL is null, using mobile web checkout URL: %s", checkoutURL)
	}
	if checkoutURL == "" {
		log.Printf("No valid checkout URL found in response")
		return nil, fmt.Errorf("no valid checkout URL provided in response")
	}

	log.Printf("Charge response: ID=%s, Status=%s, CheckoutURL=%s", chargeResp.ID, chargeResp.Status, checkoutURL)

	// Ensure status is either PENDING or SUCCEEDED
	if chargeResp.Status != "PENDING" && chargeResp.Status != "SUCCEEDED" {
		log.Printf("Invalid charge status from Xendit: %s, defaulting to PENDING", chargeResp.Status)
		chargeResp.Status = "PENDING"
	}

	// Save payment
	payment := &models.Payment{
		ID:          primitive.NewObjectID().Hex(),
		ReferenceID: referenceID,
		PayerID:     payerID,
		PayeeID:     payeeID,
		Amount:      amount,
		Title:       title,
		Description: description,
		Status:      chargeResp.Status,
		ChargeID:    chargeResp.ID,
		CheckoutURL: checkoutURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err = s.db.Collection("payments").InsertOne(ctx, payment)
	if err != nil {
		log.Printf("Failed to save payment: %v", err)
		return nil, fmt.Errorf("failed to save payment: %v", err)
	}

	log.Printf("Payment created: ID=%s, ChargeID=%s, Title=%s, Description=%s", payment.ID, payment.ChargeID, payment.Title, payment.Description)
	return payment, nil
}

func (s *PaymentService) CreateDisbursement(ctx context.Context, paymentID string) error {
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Validate paymentID
	paymentID = strings.TrimSpace(paymentID)
	paymentObjID, err := primitive.ObjectIDFromHex(paymentID)
	if err != nil {
		log.Printf("Invalid paymentID format: %s, error: %v", paymentID, err)
		return fmt.Errorf("invalid payment_id format: %v", err)
	}

	// Find the payment
	var payment models.Payment
	if err := s.db.Collection("payments").FindOne(ctx, bson.M{"_id": paymentObjID}).Decode(&payment); err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Payment not found for ID %s", paymentID)
			return fmt.Errorf("payment not found")
		}
		log.Printf("Failed to fetch payment %s: %v", paymentID, err)
		return fmt.Errorf("failed to fetch payment: %v", err)
	}

	// Ensure payment is in SUCCEEDED state before disbursement
	if payment.Status != "SUCCEEDED" {
		log.Printf("Cannot disburse payment %s with status %s", paymentID, payment.Status)
		return fmt.Errorf("can only disburse payment with status SUCCEEDED, current status is %s", payment.Status)
	}

	// Find payee
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

	// Use local account number for Xendit disbursement
	accountNumber := payee.GCashNumber
	log.Printf("Using account number for disbursement: %s", accountNumber)

	// Prepare disbursement request
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

	// Log the request body for debugging
	log.Printf("Xendit disbursement request body: %s", string(reqBody))

	// Make HTTP request to Xendit with retry logic
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
		if err == nil && resp.StatusCode == http.StatusCreated {
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
	if err != nil || resp.StatusCode != http.StatusCreated {
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

	// Ensure status is either PENDING or SUCCEEDED
	if disResp.Status != "PENDING" && disResp.Status != "SUCCEEDED" {
		log.Printf("Invalid disbursement status from Xendit: %s, defaulting to SUCCEEDED", disResp.Status)
		disResp.Status = "SUCCEEDED"
	}

	// Update payment
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
	// Set query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	eventType, ok := payload["event"].(string)
	if !ok {
		log.Printf("Invalid webhook event type")
		return fmt.Errorf("invalid webhook event type")
	}

	log.Printf("Received webhook: event=%s", eventType)

	if eventType == "ewallet.charge.created" || eventType == "ewallet.charge.updated" || eventType == "ewallet.capture" {
		data, ok := payload["data"].(map[string]interface{})
		if !ok {
			log.Printf("Invalid webhook data")
			return fmt.Errorf("invalid webhook data")
		}
		chargeID, _ := data["id"].(string)
		status, _ := data["status"].(string)

		log.Printf("Processing webhook for charge %s with status %s", chargeID, status)

		var payment models.Payment
		err := s.db.Collection("payments").FindOne(ctx, bson.M{"charge_id": chargeID}).Decode(&payment)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("Payment not found for charge %s", chargeID)
				return fmt.Errorf("payment not found for charge %s", chargeID)
			}
			log.Printf("Failed to fetch payment for charge %s: %v", chargeID, err)
			return fmt.Errorf("failed to fetch payment for charge %s: %v", chargeID, err)
		}
		log.Printf("Payment found for charge %s: ID=%s", chargeID, payment.ID)

		if status == "SUCCEEDED" {
			// Update payment status to SUCCEEDED
			_, err = s.db.Collection("payments").UpdateOne(ctx, bson.M{"charge_id": chargeID}, bson.M{
				"$set": bson.M{
					"status":     "SUCCEEDED",
					"updated_at": time.Now(),
				},
			})
			if err != nil {
				log.Printf("Failed to update payment status to SUCCEEDED for charge %s: %v", chargeID, err)
				return fmt.Errorf("failed to update payment status: %v", err)
			}
			log.Printf("Updated payment status to SUCCEEDED for charge %s", chargeID)

			// Initiate disbursement
			return s.CreateDisbursement(ctx, payment.ID)
		} else {
			// Log unexpected status but do not update to FAILED
			log.Printf("Received unexpected status %s for charge %s, no action taken", status, chargeID)
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

		// Ensure status is either PENDING or SUCCEEDED
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
