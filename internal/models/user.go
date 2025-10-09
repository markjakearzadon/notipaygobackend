package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents a user in the system
type User struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FirstName         string             `bson:"first_name" json:"first_name"`
	MiddleInitial     string             `bson:"middle_initial" json:"middle_initial"`
	LastName          string             `bson:"last_name" json:"last_name"`
	CollegeDepartment string             `bson:"college_department" json:"college_department"`
	Birthday          *time.Time         `bson:"birthday,omitempty" json:"birthday,omitempty"` // Optional
	Email             string             `bson:"email" json:"email"`
	CellphoneNumber   string             `bson:"cellphone_number" json:"cellphone_number"` // e.g., "09123456789"
	Password          string             `bson:"password" json:"password"`
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
	Role              string             `bson:"role" json:"role"`     // e.g., "admin", "user"
	Status            string             `bson:"status" json:"status"` // e.g., "pending", "active"
}
