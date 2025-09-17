package models

type Transaction struct {
	OrderID         string  `json:"orderId" bson:"orderId"`
	ChargeID        string  `json:"chargeId" bson:"chargeId"`
	Amount          float64 `json:"amount" bson:"amount"`
	RecipientMobile string  `json:"recipientMobile" bson:"recipientMobile"`
	Status          string  `json:"status" bson:"status"` // e.g., PENDING, SUCCEEDED
	DisbursementID  string  `json:"disbursementId" bson:"disbursementId"`
}
