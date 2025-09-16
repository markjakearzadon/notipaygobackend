package handlers

import (
	"encoding/json"
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

// CreateUser sdsdlj
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user models.User

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if user.FullName == "" || user.Email == "" || user.Number == "" || user.HPassword == "" {
		http.Error(w, "missing required field", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.HPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	user.HPassword = string(hashedPassword)

	id, err := h.service.CreateUser(r.Context(), &user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, err := h.service.UserList(r.Context())
	if err != nil {
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&users); err != nil {
		http.Error(w, "failed to fetch users", http.StatusInternalServerError)
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID        string    `json:"id"`
		FullName  string    `json:"fullname"`
		Email     string    `json:"email"`
		Number    string    `json:"number"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *UserHandler) LoginUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received login request: %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"Email and password are required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		if err.Error() == "user not found" || err.Error() == "invalid password" {
			http.Error(w, `{"error":"Invalid email or password"}`, status)
		} else {
			http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID.Hex(),
		"email":    user.Email,
		"fullname": user.FullName,
		"number":   user.Number,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
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
	resp.User.FullName = user.FullName
	resp.User.Email = user.Email
	resp.User.Number = user.Number
	resp.User.CreatedAt = user.CreatedAt

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
	}
}
