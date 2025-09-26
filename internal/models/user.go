package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FullName    string             `bson:"fullname" json:"fullname"`
	Email       string             `bson:"email" json:"email"`
	HPassword   string             `bson:"password" json:"password"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	GCashNumber string             `bson:"gcash_number" json:"gcash_number"` // e.g., "09123456789"
	Role        string             `bson:"role" json:"role"`                 // e.g., "admin", "user"
}
