package services

import (
	"context"
	"time"

	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type AnnouncementService struct {
	collection *mongo.Collection
}

func NewAnnouncementService(db *mongo.Database) *AnnouncementService {
	return &AnnouncementService{collection: db.Collection("announcement")}
}

func (s *AnnouncementService) CreateAnnouncement(ctx context.Context, announcement *models.Announcement) (string, error) {
	announcement.ID = primitive.NewObjectID()
	announcement.CreatedAt = time.Now()
	announcement.UpdatedAt = time.Now()

	result, err := s.collection.InsertOne(ctx, announcement)
	if err != nil {
		return "", err
	}

	return result.InsertedID.(primitive.ObjectID).Hex(), err
}

func (s *AnnouncementService) AnnouncementList(ctx context.Context) ([]models.Announcement, error) {
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

func (s *AnnouncementService) DeleteAnnouncement(ctx context.Context, id string) (string, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return "", err
	}
	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return "", err
	}

	return id, nil
}

//
//create
//getlist
//delete
