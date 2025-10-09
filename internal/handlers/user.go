package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FirstName         string  `json:"first_name"`
		MiddleInitial     string  `json:"middle_initial"`
		LastName          string  `json:"last_name"`
		CellphoneNumber   string  `json:"cellphone_number"`
		Email             string  `json:"email"`
		CollegeDepartment string  `json:"college_department"`
		Birthday          *string `json:"birthday"`
		Password          string  `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Convert birthday string to time.Time
	var birthday *time.Time
	if req.Birthday != nil {
		parsedBirthday, err := time.Parse(time.RFC3339, *req.Birthday)
		if err != nil {
			http.Error(w, `{"error":"Invalid birthday format, must be RFC3339 (e.g., 2000-01-01T00:00:00Z)"}`, http.StatusBadRequest)
			return
		}
		birthday = &parsedBirthday
	}

	user := &models.User{
		FirstName:         req.FirstName,
		MiddleInitial:     req.MiddleInitial,
		LastName:          req.LastName,
		CellphoneNumber:   req.CellphoneNumber,
		Email:             req.Email,
		CollegeDepartment: req.CollegeDepartment,
		Birthday:          birthday,
		Password:          req.Password,
	}

	userID, err := h.service.CreateUser(r.Context(), user)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		if strings.Contains(err.Error(), "is required") {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Fetch the created user to include in response
	createdUser, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to fetch created user %s: %v", userID, err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to fetch created user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"role":    createdUser.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Failed to generate JWT: %v", err)
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	// Prepare response (omit password)
	responseUser := map[string]interface{}{
		"id":                 createdUser.ID.Hex(),
		"first_name":         createdUser.FirstName,
		"middle_initial":     createdUser.MiddleInitial,
		"last_name":          createdUser.LastName,
		"cellphone_number":   createdUser.CellphoneNumber,
		"email":              createdUser.Email,
		"college_department": createdUser.CollegeDepartment,
		"birthday":           createdUser.Birthday,
		"role":               createdUser.Role,
		"status":             createdUser.Status,
		"created_at":         createdUser.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User created successfully",
		"user":    responseUser,
		"token":   tokenString,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, err := h.service.UserList(r.Context())
	if err != nil {
		log.Printf("Failed to fetch users: %v", err)
		http.Error(w, `{"error":"Failed to fetch users"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID                string    `json:"id"`
		FirstName         string    `json:"first_name"`
		MiddleInitial     string    `json:"middle_initial"`
		LastName          string    `json:"last_name"`
		CollegeDepartment string    `json:"college_department"`
		Email             string    `json:"email"`
		CellphoneNumber   string    `json:"cellphone_number"`
		CreatedAt         time.Time `json:"created_at"`
		Role              string    `json:"role"`
	} `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *UserHandler) LoginUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received login request: %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid input"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"Email and password are required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("Login failed for email %s: %v", req.Email, err)
		if err.Error() == "invalid email or password" {
			http.Error(w, `{"error":"Invalid email or password"}`, http.StatusUnauthorized)
			return
		}
		if err.Error() == "your account is pending approval. Please contact the admin for assistance" {
			http.Error(w, `{"error":"Your account is pending approval. Please contact the admin for assistance"}`, http.StatusForbidden)
			return
		}
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":            user.ID.Hex(),
		"email":              user.Email,
		"first_name":         user.FirstName,
		"middle_initial":     user.MiddleInitial,
		"last_name":          user.LastName,
		"college_department": user.CollegeDepartment,
		"cellphone_number":   user.CellphoneNumber,
		"role":               user.Role,
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Printf("Failed to generate JWT: %v", err)
		http.Error(w, `{"error":"Failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	// Prepare response
	resp := LoginResponse{
		Token: tokenString,
	}
	resp.User.ID = user.ID.Hex()
	resp.User.FirstName = user.FirstName
	resp.User.MiddleInitial = user.MiddleInitial
	resp.User.LastName = user.LastName
	resp.User.CollegeDepartment = user.CollegeDepartment
	resp.User.Email = user.Email
	resp.User.CellphoneNumber = user.CellphoneNumber
	resp.User.CreatedAt = user.CreatedAt
	resp.User.Role = user.Role

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

func (h *UserHandler) UpdateUserStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Decode request body
	var input struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"Invalid input"}`, http.StatusBadRequest)
		return
	}

	if input.ID == "" {
		http.Error(w, `{"error":"User ID is required"}`, http.StatusBadRequest)
		return
	}

	// Call service to toggle status
	err := h.service.UpdateUserStatus(r.Context(), input.ID)
	if err != nil {
		log.Printf("Failed to update user status %s: %v", input.ID, err)
		if err.Error() == "user not found" {
			http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
			return
		}
		if err.Error() == "invalid user ID" {
			http.Error(w, `{"error":"Invalid user ID"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"Failed to update status"}`, http.StatusInternalServerError)
		return
	}

	// Fetch the updated user to get the new status
	user, err := h.service.GetUser(r.Context(), input.ID)
	if err != nil {
		log.Printf("Failed to fetch updated user %s: %v", input.ID, err)
		http.Error(w, `{"error":"Failed to fetch updated user"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User status updated successfully",
		"status":  user.Status,
		"user": map[string]interface{}{
			"id":                 user.ID.Hex(),
			"first_name":         user.FirstName,
			"middle_initial":     user.MiddleInitial,
			"last_name":          user.LastName,
			"college_department": user.CollegeDepartment,
			"email":              user.Email,
			"cellphone_number":   user.CellphoneNumber,
			"birthday":           user.Birthday,
			"role":               user.Role,
			"status":             user.Status,
			"created_at":         user.CreatedAt,
		},
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}
