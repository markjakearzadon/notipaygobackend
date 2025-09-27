package models

import (
	"time"
)

type Payment struct {
	ID             string    `bson:"_id,omitempty" json:"id"`
	ReferenceID    string    `bson:"reference_id" json:"reference_id"`
	PayerNumber    string    `bson:"payer_number" json:"payer_number"` // Changed from payer_id
	PayeeNumber    string    `bson:"payee_number" json:"payee_number"` // Changed from payee_id
	Amount         float64   `bson:"amount" json:"amount"`
	Title          string    `bson:"title" json:"title"`             // Payment title
	Description    string    `bson:"description" json:"description"` // Payment description
	Status         string    `bson:"status" json:"status"`           // e.g., "PENDING", "COMPLETED", "FAILED"
	ChargeID       string    `bson:"charge_id" json:"charge_id"`
	DisbursementID string    `bson:"disbursement_id" json:"disbursement_id"`
	CheckoutURL    string    `bson:"checkout_url" json:"checkout_url"` // For frontend redirect
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
}
