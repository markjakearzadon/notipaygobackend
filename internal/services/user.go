package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	indexModels := []mongo.IndexModel{
		{Keys: bson.M{"email": 1}, Options: options.Index().SetUnique(true)},
		{Keys: bson.M{"cellphone_number": 1}, Options: options.Index().SetUnique(true)},
	}
	_, err := collection.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		log.Printf("Failed to create user indexes: %v", err)
	} else {
		log.Println("User indexes created successfully")
	}
	return &UserService{collection: collection}
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Validate required fields
	if user.FirstName == "" {
		return "", errors.New("first name is required")
	}
	if user.LastName == "" {
		return "", errors.New("last name is required")
	}
	if user.Email == "" {
		return "", errors.New("email is required")
	}
	if user.CellphoneNumber == "" {
		return "", errors.New("cellphone number is required")
	}
	if user.Password == "" {
		return "", errors.New("password is required")
	}

	// Validate formats
	if len(user.FirstName) > 50 {
		return "", errors.New("first name must not exceed 50 characters")
	}
	if len(user.LastName) > 50 {
		return "", errors.New("last name must not exceed 50 characters")
	}
	if len(user.MiddleInitial) > 5 {
		return "", errors.New("middle initial must not exceed 5 characters")
	}
	if len(user.CollegeDepartment) > 100 {
		return "", errors.New("college department must not exceed 100 characters")
	}
	if !strings.HasPrefix(user.CellphoneNumber, "0") || len(user.CellphoneNumber) != 11 {
		return "", errors.New("cellphone number must start with 0 and be 11 digits")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`).MatchString(user.Email) {
		return "", errors.New("invalid email format")
	}
	if user.Birthday != nil && user.Birthday.After(time.Now()) {
		return "", errors.New("birthday cannot be in the future")
	}

	// Check for unique email and cellphone number
	count, err := s.collection.CountDocuments(ctx, bson.M{"email": user.Email})
	if err != nil {
		log.Printf("Failed to check email uniqueness: %v", err)
		return "", fmt.Errorf("failed to check email uniqueness: %v", err)
	}
	if count > 0 {
		return "", errors.New("email already exists")
	}

	count, err = s.collection.CountDocuments(ctx, bson.M{"cellphone_number": user.CellphoneNumber})
	if err != nil {
		log.Printf("Failed to check cellphone number uniqueness: %v", err)
		return "", fmt.Errorf("failed to check cellphone number uniqueness: %v", err)
	}
	if count > 0 {
		return "", errors.New("cellphone number already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		return "", fmt.Errorf("failed to hash password: %v", err)
	}
	user.Password = string(hashedPassword)

	// Set defaults and timestamps
	user.ID = primitive.NewObjectID()
	user.CreatedAt = time.Now()
	if user.Role == "" {
		user.Role = "user"
	}
	if user.Status == "" {
		user.Status = "pending"
	}

	// Insert user into database
	result, err := s.collection.InsertOne(ctx, user)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		return "", fmt.Errorf("failed to create user: %v", err)
	}

	userID := result.InsertedID.(primitive.ObjectID).Hex()
	log.Printf("User created: ID=%s, Email=%s, CellphoneNumber=%s", userID, user.Email, user.CellphoneNumber)
	return userID, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

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
		log.Printf("Failed to get user %s: %v", id, err)
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	return &user, nil
}

func (s *UserService) UserList(ctx context.Context) ([]models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	projection := bson.M{
		"password": 0, // Exclude password field
	}
	cur, err := s.collection.Find(ctx, bson.M{}, options.Find().SetProjection(projection))
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		return nil, fmt.Errorf("failed to list users: %v", err)
	}

	var users []models.User
	defer cur.Close(ctx)
	if err := cur.All(ctx, &users); err != nil {
		log.Printf("Failed to decode users: %v", err)
		return nil, fmt.Errorf("failed to decode users: %v", err)
	}

	return users, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return "", errors.New("invalid user ID")
	}

	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		log.Printf("Failed to delete user %s: %v", id, err)
		return "", fmt.Errorf("failed to delete user: %v", err)
	}

	if result.DeletedCount == 0 {
		return "", errors.New("user not found")
	}

	log.Printf("User deleted: ID=%s", id)
	return id, nil
}

func (s *UserService) Login(ctx context.Context, email, password string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		log.Printf("Failed to find user by email %s: %v", email, err)
		return nil, fmt.Errorf("failed to find user: %v", err)
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

func (s *UserService) UpdateUserStatus(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid user ID")
	}

	var user models.User
	err = s.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user not found")
		}
		log.Printf("Failed to find user %s: %v", id, err)
		return fmt.Errorf("failed to find user: %v", err)
	}

	newStatus := "active"
	if user.Status == "active" {
		newStatus = "pending"
	}

	update := bson.M{
		"$set": bson.M{
			"status":     newStatus,
			"updated_at": time.Now(),
		},
	}

	result, err := s.collection.UpdateOne(ctx, bson.M{"_id": objID}, update)
	if err != nil {
		log.Printf("Failed to update user status %s: %v", id, err)
		return fmt.Errorf("failed to update user status: %v", err)
	}

	if result.MatchedCount == 0 {
		return errors.New("user not found")
	}

	log.Printf("User status updated: ID=%s, NewStatus=%s", id, newStatus)
	return nil
}
