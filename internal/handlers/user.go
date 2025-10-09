package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("myjwtsecretkey")

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// CreateUser creates a new user with required fields including role
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		models.User
		ConfirmPassword string `json:"confirm_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"Invalid input"}`, http.StatusBadRequest)
		return
	}

	if input.FirstName == "" || input.LastName == "" || input.Email == "" || input.CellphoneNumber == "" || input.Password == "" || input.Role == "" {
		http.Error(w, `{"error":"Missing required fields: first_name, last_name, email, cellphone_number, password, and role are required"}`, http.StatusBadRequest)
		return
	}

	if input.Password != input.ConfirmPassword {
		http.Error(w, `{"error":"Passwords do not match"}`, http.StatusBadRequest)
		return
	}

	// Validate role
	if input.Role != "admin" && input.Role != "user" {
		http.Error(w, `{"error":"Invalid role: must be 'admin' or 'user'"}`, http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	input.User.Password = string(hashedPassword)

	id, err := h.service.CreateUser(r.Context(), &input.User)
	if err != nil {
		if err.Error() == "email already exists" {
			http.Error(w, `{"error":"Email already exists"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"Failed to create user"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, err := h.service.UserList(r.Context())
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch users"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&users); err != nil {
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
		"exp":                time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
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
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}

// UpdateUserStatusHandler toggles the status of a user between "pending" and "active".
// Only accessible to users with the "admin" role.
func (h *UserHandler) UpdateUserStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Verify JWT and check for admin role
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, `{"error":"Authorization token required"}`, http.StatusUnauthorized)
		return
	}
	// Remove "Bearer " prefix if present
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] != "admin" {
		http.Error(w, `{"error":"Admin access required"}`, http.StatusForbidden)
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
	err = h.service.UpdateUserStatus(r.Context(), input.ID)
	if err != nil {
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
		http.Error(w, `{"error":"Failed to fetch updated user"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
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
			"created_at":         user.CreatedAt,
			"role":               user.Role,
		},
	})
}
