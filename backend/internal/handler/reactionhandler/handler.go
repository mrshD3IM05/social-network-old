package reactionhandler

import (
	"errors"
	"net/http"

	"sn-backend/internal/handler/common"
	"sn-backend/internal/service/reactionsvc"
	"sn-backend/internal/service/sessionsvc"
)

type Handler struct {
	Service *reactionsvc.Service
	Session *sessionsvc.Service
}

func New(service *reactionsvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Session: session}
}

func (h *Handler) CreateReaction(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	postID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	summary, err := h.Service.React(userID, postID, r.FormValue("reaction"))
	if err != nil {
		writeError(w, err, "could not react to post")
		return
	}
	common.WriteJSON(w, http.StatusOK, summary)
}

func (h *Handler) DeleteReaction(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	postID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	summary, err := h.Service.Unreact(userID, postID)
	if err != nil {
		writeError(w, err, "could not remove reaction")
		return
	}
	common.WriteJSON(w, http.StatusOK, summary)
}

func writeError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, reactionsvc.ErrInvalidReaction):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, reactionsvc.ErrPostNotFound):
		http.Error(w, "post not found", http.StatusNotFound)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}
