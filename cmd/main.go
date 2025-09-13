package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/db"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/handlers"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("error loading .env %s", err)
	}

	uri := os.Getenv("MONGOURI")
	if err := db.Connect(uri); err != nil {
		log.Fatalf("failed to connect %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer db.Disconnect(ctx)

	err = db.Client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	notidatabase := db.Client.Database("notipaydb")
	userService := services.NewUserService(notidatabase)
	userHandler := handlers.NewUserHandler(userService)

	// Set up router
	router := mux.NewRouter()
	router.HandleFunc("/user", userHandler.CreateUser).Methods("POST")
	router.HandleFunc("/user", userHandler.GetUsers).Methods("GET")

	// Start server
	log.Println("Server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
