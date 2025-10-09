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
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	collection *mongo.Collection
}

func NewUserService(db *mongo.Database) *UserService {
	collection := db.Collection("user")
	_, err := collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.M{"email": 1},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return &UserService{collection: collection}
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) (string, error) {
	// Validate required fields
	if user.FirstName == "" || user.LastName == "" || user.Email == "" || user.CellphoneNumber == "" || user.Password == "" {
		return "", errors.New("missing required fields: first_name, last_name, email, cellphone_number, and password are required")
	}

	count, err := s.collection.CountDocuments(ctx, bson.M{"email": user.Email})
	if err != nil {
		return "", err
	}

	if count > 0 {
		return "", errors.New("email already exists")
	}

	user.ID = primitive.NewObjectID()
	user.CreatedAt = time.Now()
	// Set default role to "user" if not provided
	if user.Role == "" {
		user.Role = "user"
	}

	if user.Status == "" {
		user.Status = "pending"
	}

	result, err := s.collection.InsertOne(ctx, user)
	if err != nil {
		return "", err
	}

	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

// GetUser by id of type string
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid user ID")
	}

	var user models.User
	err = s.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (s *UserService) UserList(ctx context.Context) ([]models.User, error) {
	option := bson.D{
		{Key: "password", Value: 0},
	}
	cur, err := s.collection.Find(ctx, bson.D{}, options.Find().SetProjection(option))
	if err != nil {
		return nil, err
	}

	var users []models.User
	defer cur.Close(ctx)

	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

// DeleteUser removes a user document from the database by its id (string)
// Parameters:
//   - ctx: Context for the database operation.
//   - id: String representation of the user's ObjectID.
//
// Returns:
//   - string: The ID of the deleted user if successful.
//   - error: Nil if successful, or an error if the operation fails.
func (s *UserService) DeleteUser(ctx context.Context, id string) (string, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return "", errors.New("invalid user ID")
	}

	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return "", err
	}

	if result.DeletedCount == 0 {
		return "", errors.New("user not found")
	}

	return id, nil
}

func (s *UserService) Login(ctx context.Context, email, password string) (*models.User, error) {
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	if user.Status == "pending" {
		return nil, errors.New("your account is pending approval. Please contact the admin for assistance")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	return &user, nil
}

// UpdateUserStatus toggles the status of a user identified by their ID between "pending" and "active".
// Parameters:
//   - ctx: Context for the database operation.
//   - id: String representation of the user's ObjectID.
//
// Returns:
//   - error: Nil if successful, or an error if the operation fails.
func (s *UserService) UpdateUserStatus(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid user ID")
	}

	// Fetch the current user to check their status
	var user models.User
	err = s.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user not found")
		}
		return err
	}

	// Toggle status between "pending" and "active"
	newStatus := "active"
	if user.Status == "active" {
		newStatus = "pending"
	}

	update := bson.M{
		"$set": bson.M{
			"status": newStatus,
		},
	}

	result, err := s.collection.UpdateOne(ctx, bson.M{"_id": objID}, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	return nil
}
