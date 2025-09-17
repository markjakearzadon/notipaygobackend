package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
	"go.mongodb.org/mongo-driver/bson"
)

type XenditHandler struct {
	service *services.XenditService
}

func NewXenditHandler(service *services.XenditService) *XenditHandler {
	return &XenditHandler{service: service}
}

type PaymentRequest struct {
	Amount          float64 `json:"amount"`
	RecipientMobile string  `json:"recipientMobile"`
	OrderID         string  `json:"orderId"`
}

func (h *XenditHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chargeID, checkoutURL, err := h.service.CreatePayment(ctx, req.OrderID, req.Amount, req.RecipientMobile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"chargeId":    chargeID,
		"checkoutUrl": checkoutURL,
	})
}

func (h *XenditHandler) CheckPaymentStatus(w http.ResponseWriter, r *http.Request) {
	chargeID := mux.Vars(r)["chargeId"]
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := h.service.CheckPaymentStatus(ctx, chargeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (h *XenditHandler) CreateDisbursement(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	disbursementID, err := h.service.CreateDisbursement(ctx, req.OrderID, req.Amount, req.RecipientMobile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"disbursementId": disbursementID})
}

func (h *XenditHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Data struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			ReferenceID string `json:"reference_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid webhook", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if payload.Data.Status == "SUCCEEDED" {
		// Update MongoDB using exported Collection field
		_, err := h.service.Collection.UpdateOne(ctx, bson.M{"chargeId": payload.Data.ID}, bson.M{"$set": bson.M{"status": payload.Data.Status}})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
