package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AnnouncementHandler handles HTTP requests for announcements
type AnnouncementHandler struct {
	announcementService *services.AnnouncementService
}

// NewAnnouncementHandler creates a new AnnouncementHandler
func NewAnnouncementHandler(announcementService *services.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{announcementService: announcementService}
}

// CreateAnnouncement handles POST /api/announcement
func (h *AnnouncementHandler) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var announcement models.Announcement
	if err := json.NewDecoder(r.Body).Decode(&announcement); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create announcement in the database
	id, err := h.announcementService.CreateAnnouncement(r.Context(), &announcement)
	if err != nil {
		if err.Error() == "announcement with this title already exists" {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to create announcement: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	announcement.ID = id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(announcement)
}

// GetAnnouncements handles GET /api/announcements
func (h *AnnouncementHandler) GetAnnouncements(w http.ResponseWriter, r *http.Request) {
	announcements, err := h.announcementService.GetAnnouncements(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve announcements: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(announcements)
}

// GetAnnouncement handles GET /api/announcement/{announcementID}
func (h *AnnouncementHandler) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["announcementID"])
	if err != nil {
		http.Error(w, "Invalid announcement ID", http.StatusBadRequest)
		return
	}

	announcement, err := h.announcementService.GetAnnouncementByID(r.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Announcement not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve announcement: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(announcement)
}

// UpdateAnnouncement handles PATCH /api/announcement/{announcementID}
func (h *AnnouncementHandler) UpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["announcementID"])
	if err != nil {
		http.Error(w, "Invalid announcement ID", http.StatusBadRequest)
		return
	}

	var announcement models.Announcement
	if err := json.NewDecoder(r.Body).Decode(&announcement); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	announcement.ID = id
	if err := h.announcementService.UpdateAnnouncement(r.Context(), &announcement); err != nil {
		if err.Error() == "another announcement with this title already exists" {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to update announcement: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(announcement)
}

// DeleteAnnouncement handles DELETE /api/announcement/{announcementID}
func (h *AnnouncementHandler) DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := primitive.ObjectIDFromHex(vars["announcementID"])
	if err != nil {
		http.Error(w, "Invalid announcement ID", http.StatusBadRequest)
		return
	}

	if err := h.announcementService.DeleteAnnouncement(r.Context(), id); err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Announcement not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete announcement: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
