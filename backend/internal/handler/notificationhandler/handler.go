package notificationhandler

import (
	"net/http"

	"sn-backend/internal/handler/common"
	"sn-backend/internal/service/notificationsvc"
	"sn-backend/internal/service/sessionsvc"
)

type Handler struct {
	Service *notificationsvc.Service
	Session *sessionsvc.Service
}

func New(service *notificationsvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Session: session}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	notifications, err := h.Service.List(userID, common.LastID(r))
	if err != nil {
		http.Error(w, "could not list notifications", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, http.StatusOK, notifications)
}

func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	count, err := h.Service.UnreadCount(userID)
	if err != nil {
		http.Error(w, "could not count notifications", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if err := h.Service.MarkRead(userID); err != nil {
		http.Error(w, "could not update notifications", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
