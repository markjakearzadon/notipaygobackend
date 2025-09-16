package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/markjakearzadon/notipay-gobackend.git/internal/models"
	"github.com/markjakearzadon/notipay-gobackend.git/internal/services"
)

type AnnouncementHandler struct {
	service *services.AnnouncementService
}

func NewAnnouncementHandler(service *services.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{service: service}
}

func (h *AnnouncementHandler) CreateAnnouncementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var announcement models.Announcement

	if err := json.NewDecoder(r.Body).Decode(&announcement); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	id, err := h.service.CreateAnnouncement(r.Context(), &announcement)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *AnnouncementHandler) DeleteAnnouncementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed bro", http.StatusMethodNotAllowed)
		return
	}
	idontknow := strings.Split(r.URL.Path, "/")

	if len(idontknow) < 3 {
		http.Error(w, "invalid url: missing announcement id", http.StatusBadRequest)
	}
	id := idontknow[len(idontknow)-1]

	deletedID, err := h.service.DeleteAnnouncement(r.Context(), id)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete announcement: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": deletedID})
}

func (h *AnnouncementHandler) AnnouncementListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	announcements, err := h.service.AnnouncementList(r.Context())
	if err != nil {
		http.Error(w, "failed to fetch announcments", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&announcements); err != nil {
		http.Error(w, "failed to fetch users", http.StatusInternalServerError)
	}
}
