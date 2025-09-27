package services

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AnnouncementService handles database operations for announcements
type AnnouncementService struct {
	collection *mongo.Collection
}

// NewAnnouncementService initializes a new AnnouncementService
func NewAnnouncementService(db *mongo.Database) *AnnouncementService {
	collection := db.Collection("announcement")
	_, err := collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.M{"title": 1},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Fatalf("error creating index for announcement: %v", err)
	}
	return &AnnouncementService{collection: collection}
}

// CreateAnnouncement creates a new announcement in the database
func (s *AnnouncementService) CreateAnnouncement(ctx context.Context, announcement *models.Announcement) (primitive.ObjectID, error) {
	count, err := s.collection.CountDocuments(ctx, bson.M{"title": announcement.Title})
	if err != nil {
		return primitive.ObjectID{}, err
	}

	if count > 0 {
		return primitive.ObjectID{}, errors.New("announcement with this title already exists")
	}

	announcement.ID = primitive.NewObjectID()
	announcement.CreatedAt = time.Now()
	announcement.UpdatedAt = time.Now()

	result, err := s.collection.InsertOne(ctx, announcement)
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return result.InsertedID.(primitive.ObjectID), nil
}

// GetAnnouncements retrieves all announcements from the database
func (s *AnnouncementService) GetAnnouncements(ctx context.Context) ([]models.Announcement, error) {
	cur, err := s.collection.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}

	var announcements []models.Announcement
	defer cur.Close(ctx)

	if err := cur.All(ctx, &announcements); err != nil {
		return nil, err
	}

	return announcements, nil
}

// GetAnnouncementByID retrieves an announcement by its ID
func (s *AnnouncementService) GetAnnouncementByID(ctx context.Context, id primitive.ObjectID) (*models.Announcement, error) {
	var announcement models.Announcement
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&announcement)
	if err != nil {
		return nil, err
	}

	return &announcement, nil
}

// UpdateAnnouncement updates an existing announcement
func (s *AnnouncementService) UpdateAnnouncement(ctx context.Context, announcement *models.Announcement) error {
	// Check if another announcement with the same title exists (excluding the current announcement)
	count, err := s.collection.CountDocuments(ctx, bson.M{
		"title": announcement.Title,
		"_id":   bson.M{"$ne": announcement.ID},
	})
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("another announcement with this title already exists")
	}

	announcement.UpdatedAt = time.Now()
	update := bson.M{
		"$set": bson.M{
			"title":      announcement.Title,
			"content":    announcement.Content,
			"updated_at": announcement.UpdatedAt,
		},
	}

	_, err = s.collection.UpdateOne(ctx, bson.M{"_id": announcement.ID}, update)
	return err
}

// DeleteAnnouncement removes an announcement by its ID
func (s *AnnouncementService) DeleteAnnouncement(ctx context.Context, id primitive.ObjectID) error {
	_, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
