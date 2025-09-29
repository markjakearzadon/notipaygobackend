package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/handlers"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dateValue = "2025-09-29" // default
	dateMu    sync.RWMutex
)

func GetDateHandler(w http.ResponseWriter, r *http.Request) {
	dateMu.RLock()
	response := map[string]string{
		"date": dateValue,
	}
	dateMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func UpdateDateHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.Date == "" {
		http.Error(w, "date field is required", http.StatusBadRequest)
		return
	}

	dateMu.Lock()
	dateValue = body.Date
	dateMu.Unlock()

	response := map[string]string{
		"message": "Date updated successfully",
		"date":    dateValue,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Load .env
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Error loading .env: %s", err)
	}

	// Connect to MongoDB
	uri := os.Getenv("MONGOURI")
	if uri == "" {
		log.Fatal("MONGOURI environment variable not set")
	}
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Println("Successfully connected to MongoDB")

	notidatabase := client.Database("notipaydb")

	// Initialize services and handlers
	userService := services.NewUserService(notidatabase)
	userHandler := handlers.NewUserHandler(userService)

	paymentService := services.NewPaymentService(notidatabase)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

	announcementService := services.NewAnnouncementService(notidatabase)
	announcementHandler := handlers.NewAnnouncementHandler(announcementService)

	// Set up router
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET", "HEAD")
	router.HandleFunc("/api/user", userHandler.CreateUser).Methods("POST")
	router.HandleFunc("/api/user", userHandler.GetUsers).Methods("GET")
	router.HandleFunc("/api/login", userHandler.LoginUserHandler).Methods("POST")

	router.HandleFunc("/api/announcement", announcementHandler.CreateAnnouncement).Methods("POST")
	router.HandleFunc("/api/announcements", announcementHandler.GetAnnouncements).Methods("GET")
	router.HandleFunc("/api/announcement/{announcementID}", announcementHandler.GetAnnouncement).Methods("GET")
	router.HandleFunc("/api/announcement/{announcementID}", announcementHandler.UpdateAnnouncement).Methods("PATCH")
	router.HandleFunc("/api/announcement/{announcementID}", announcementHandler.DeleteAnnouncement).Methods("DELETE")

	router.HandleFunc("/api/payment", paymentHandler.CreatePayment).Methods("POST")
	router.HandleFunc("/api/payments", paymentHandler.GetPayments).Methods("GET")
	router.HandleFunc("/api/payment/webhook", paymentHandler.Webhook).Methods("POST")
	router.HandleFunc("/api/updatepayment/{paymentID}", paymentHandler.UpdatePayment).Methods("PATCH", "PUT", "GET")
	router.HandleFunc("/api/userid/{userID}/payments", paymentHandler.GetPaymentsByUserID).Methods("GET")
	router.HandleFunc("/api/payment/{paymentID}", paymentHandler.GetPaymentHandler).Methods("GET")
	router.HandleFunc("/api/bulk-payment", paymentHandler.CreateBulkPayment).Methods("POST")

	// Date endpoints
	router.HandleFunc("/api/date", GetDateHandler).Methods("GET")
	router.HandleFunc("/api/date", UpdateDateHandler).Methods("PUT", "PATCH")

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:         "0.0.0.0:" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("Server running on port %s", port)
	log.Fatal(server.ListenAndServe())
}
