package filehandler

import (
	"net/http"
	"sn-backend/internal/handler/common"
	"sn-backend/internal/service/filesvc"
	"sn-backend/internal/service/sessionsvc"
)

type Handler struct {
	Service *filesvc.Service
	Session *sessionsvc.Service
}

func New(service *filesvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Session: session}
}

func (h *Handler) SetAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, filesvc.MaxImageSize+1<<20)
	if err := r.ParseMultipartForm(filesvc.MaxMemory); err != nil {
		http.Error(w, "upload is too large or invalid", http.StatusBadRequest)
		return
	}
	headers := r.MultipartForm.File["avatar"]
	if len(headers) != 1 {
		http.Error(w, "exactly one avatar image is required", http.StatusBadRequest)
		return
	}
	user, err := h.Service.SetAvatar(userID, headers[0])
	if err != nil {
		if filesvc.IsBadImage(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "could not set avatar", http.StatusInternalServerError)
		}
		return
	}
	common.WriteJSON(w, http.StatusOK, common.PrivateUser(user))
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	ownerID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	file, err := h.Service.Get(id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	visible, err := h.Service.CanView(ownerID, id)
	if err != nil || !visible {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", file.MIMEType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	// the browser must treat it as the stored image type and never run it
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeFile(w, r, file.StoragePath)
}
