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

	announcementService := services.NewAnnouncementService(notidatabase)
	announcementHandler := handlers.NewAnnouncementHandler(announcementService)

	// Set up router
	router := mux.NewRouter()
	router.HandleFunc("/api/user", userHandler.CreateUser).Methods("POST")
	router.HandleFunc("/api/user", userHandler.GetUsers).Methods("GET")
	router.HandleFunc("/api/login", userHandler.LoginUserHandler).Methods("POST")
	router.HandleFunc("/api/announcement", announcementHandler.CreateAnnouncementHandler).Methods("POST")
	router.HandleFunc("/api/announcement", announcementHandler.AnnouncementListHandler).Methods("GET")

	// Start server
	log.Println("Server running on port 42069")
	log.Fatal(http.ListenAndServe("0.0.0.0:42069", router))
}
