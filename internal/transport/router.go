package transport

import (
	"encoding/json"
	"net/http"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/service"
)

type Handler struct {
	service *service.NotificationService
}

func NewHandler(s *service.NotificationService) *Handler {
	return &Handler{service: s}
}

func (h *Handler) Router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /notifications", h.createNotification)
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type createNotificationRequest struct {
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
}

func (h *Handler) createNotification(w http.ResponseWriter, r *http.Request) {
	var req createNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	n, err := h.service.Create(notification.Channel(req.Channel), req.Recipient, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(n)
}
