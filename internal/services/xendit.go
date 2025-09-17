package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type XenditService struct {
	Collection *mongo.Collection // Changed from collection to Collection
	secretKey  string
	baseURL    string
}

type XenditPaymentRequest struct {
	ReferenceID       string  `json:"reference_id"`
	Currency          string  `json:"currency"`
	Amount            float64 `json:"amount"`
	CheckoutMethod    string  `json:"checkout_method"`
	ChannelCode       string  `json:"channel_code"`
	ChannelProperties struct {
		SuccessRedirectURL string `json:"success_redirect_url"`
		FailureRedirectURL string `json:"failure_redirect_url"`
	} `json:"channel_properties"`
}

type XenditPaymentResponse struct {
	ID          string `json:"id"`
	CheckoutURL string `json:"actions"`
}

type XenditDisbursementRequest struct {
	ExternalID  string  `json:"external_id"`
	Amount      float64 `json:"amount"`
	Destination struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"destination"`
	Description string `json:"description"`
}

type XenditDisbursementResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type XenditChargeStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func NewXenditService(db *mongo.Database) *XenditService {
	return &XenditService{
		Collection: db.Collection("transactions"),
		secretKey:  os.Getenv("XENDIT_SECRET_KEY"),
		baseURL:    os.Getenv("XENDIT_BASE_URL"),
	}
}

func (s *XenditService) CreatePayment(ctx context.Context, orderID string, amount float64, recipientMobile string) (string, string, error) {
	reqBody := XenditPaymentRequest{
		ReferenceID:    orderID,
		Currency:       "PHP",
		Amount:         amount,
		CheckoutMethod: "ONE_TIME_PAYMENT",
		ChannelCode:    "PH_GCASH",
	}
	reqBody.ChannelProperties.SuccessRedirectURL = "yourapp://success?orderId=" + orderID
	reqBody.ChannelProperties.FailureRedirectURL = "yourapp://failure?orderId=" + orderID

	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/ewallets/charges", bytes.NewBuffer(bodyBytes))
	req.SetBasicAuth(s.secretKey, "")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", "", errors.New("Xendit error: " + string(body))
	}

	var result struct {
		ID      string `json:"id"`
		Actions struct {
			CheckoutURL string `json:"checkout_url"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	// Save transaction to MongoDB
	_, err = s.Collection.InsertOne(ctx, bson.M{
		"orderId":         orderID,
		"chargeId":        result.ID,
		"amount":          amount,
		"recipientMobile": recipientMobile,
		"status":          "PENDING",
	})
	if err != nil {
		return "", "", err
	}

	return result.ID, result.Actions.CheckoutURL, nil
}

func (s *XenditService) CheckPaymentStatus(ctx context.Context, chargeID string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/ewallets/charges/"+chargeID, nil)
	req.SetBasicAuth(s.secretKey, "")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var status XenditChargeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", err
	}

	// Update status in MongoDB
	_, err = s.Collection.UpdateOne(ctx, bson.M{"chargeId": chargeID}, bson.M{"$set": bson.M{"status": status.Status}})
	return status.Status, err
}

func (s *XenditService) CreateDisbursement(ctx context.Context, orderID string, amount float64, recipientMobile string) (string, error) {
	reqBody := XenditDisbursementRequest{
		ExternalID:  "DISB-" + orderID,
		Amount:      amount - 2, // Simulate fee
		Description: "P2P Test Transfer",
	}
	reqBody.Destination.Type = "PHONE_NUMBER"
	reqBody.Destination.Value = recipientMobile

	bodyBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/disbursements", bytes.NewBuffer(bodyBytes))
	req.SetBasicAuth(s.secretKey, "")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.New("Xendit error: " + string(body))
	}

	var result XenditDisbursementResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Update MongoDB with disbursement ID
	_, err = s.Collection.UpdateOne(ctx, bson.M{"orderId": orderID}, bson.M{"$set": bson.M{"disbursementId": result.ID}})
	return result.ID, err
}
