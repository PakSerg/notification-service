package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PakSerg/NotiHub/internal/notification"
	"github.com/PakSerg/NotiHub/internal/repository"
	"github.com/PakSerg/NotiHub/internal/service"
)

type Handler struct {
	service *service.NotificationService
	limiter RateLimiter
}

// NewHandler builds a Handler. limiter may be nil, in which case requests
// are not rate limited.
func NewHandler(s *service.NotificationService, limiter RateLimiter) *Handler {
	return &Handler{service: s, limiter: limiter}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /notifications", h.createNotification)
	mux.HandleFunc("GET /notifications/{id}", h.getNotification)

	if h.limiter == nil {
		return mux
	}
	return rateLimitMiddleware(h.limiter, mux)
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

	n, err := h.service.Create(r.Context(), notification.Channel(req.Channel), req.Recipient, req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(n)
}

func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	n, err := h.service.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, "notification not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n)
}
