package services

import (
	"context"
	"errors"
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
	return &UserService{collection: db.Collection("user")}
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) (string, error) {
	user.ID = primitive.NewObjectID()
	user.CreatedAt = time.Now()

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
		return nil, err
	}

	var user models.User
	err = s.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, err
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
// parameter
// (context, id)
// returns:
//   - string: the id of the deleted user if the operation was successful.
//   - error
func (s *UserService) DeleteUser(ctx context.Context, id string) (string, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return "", err
	}

	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": objID})

	return id, err
}

func (s *UserService) Login(ctx context.Context, email, password string) (*models.User, error) {
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HPassword), []byte(password))
	if err != nil {
		return nil, errors.New("invalid password")
	}

	return &user, nil
}

// createUser
// deleteUser
// userList
// getUserById
// login
