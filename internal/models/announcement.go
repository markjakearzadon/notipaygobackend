package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Announcement represents an announcement document in the MongoDB database
type Announcement struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title     string             `bson:"title" json:"title"`
	Content   string             `bson:"content" json:"content"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}
