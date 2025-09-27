package models

import (
	"time"
)

type Payment struct {
	ID             string    `bson:"_id,omitempty" json:"id"`
	ReferenceID    string    `bson:"reference_id" json:"reference_id"`
	PayerID        string    `bson:"payer_id" json:"payer_id"`
	PayeeID        string    `bson:"payee_id" json:"payee_id"`
	Amount         float64   `bson:"amount" json:"amount"`
	Title          string    `bson:"title" json:"title"`
	Description    string    `bson:"description" json:"description"`
	Status         string    `bson:"status" json:"status"` // e.g., "PENDING", "PAID", "SETTLED", "EXPIRED"
	InvoiceID      string    `bson:"invoice_id" json:"invoice_id"`
	DisbursementID string    `bson:"disbursement_id" json:"disbursement_id"`
	CheckoutURL    string    `bson:"checkout_url" json:"checkout_url"`
	CreatedAt      time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updated_at"`
}
