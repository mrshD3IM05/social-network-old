package commenthandler

import (
	"errors"
	"net/http"

	"sn-backend/internal/handler/common"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/commentsvc"
	"sn-backend/internal/service/filesvc"
	"sn-backend/internal/service/sessionsvc"
)

type Handler struct {
	Service *commentsvc.Service
	Files   *filesvc.Service
	Session *sessionsvc.Service
}

func New(service *commentsvc.Service, files *filesvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Files: files, Session: session}
}

// ListComments handles GET /posts/{id}/comments. The viewer must be able to
// see the post (post privacy, or group membership for group posts).
func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
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
	comments, err := h.Service.List(userID, postID, common.LastID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusOK, comments)
}

// CreateComment handles POST /posts/{id}/comments with a `content` form
// field. Authorization goes through the post visibility rules in the service.
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
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
	headers, err := common.ReadFormWithFiles(w, r, filesvc.MaxRequestSize, filesvc.MaxMemory)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := filesvc.CheckImages(headers); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	comment, err := h.Service.Create(userID, postID, r.FormValue("content"), len(headers) > 0)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(headers) > 0 {
		stored, err := h.Files.UploadMany(userID, headers, nil, nil, &comment.ID)
		if err != nil {
			_ = h.Service.Delete(userID, comment.ID)
			status := http.StatusInternalServerError
			if filesvc.IsBadImage(err) {
				status = http.StatusBadRequest
			}
			http.Error(w, "could not store comment images", status)
			return
		}
		for _, file := range stored {
			comment.Images = append(comment.Images, file.ID)
		}
	}
	h.Service.NotifyCreated(userID, postID, comment)
	common.WriteJSON(w, http.StatusCreated, comment)
}

// writeError maps comment errors like the other handlers. Posts the viewer
// cannot see answer 404 — the same convention as reacting to an invisible
// post — so the API does not reveal hidden posts exist.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, commentsvc.ErrInvalidContent):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, commentsvc.ErrNoAccess),
		errors.Is(err, commentsvc.ErrNotFound),
		errors.Is(err, repository.ErrNotFound):
		http.Error(w, "post not found", http.StatusNotFound)
	default:
		http.Error(w, "could not process comment", http.StatusInternalServerError)
	}
}

// UpdateComment handles PUT /comments/{id}: the author rewrites their comment.
func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	commentID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	comment, err := h.Service.Update(userID, commentID, r.FormValue("content"))
	if err != nil {
		writeError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusOK, comment)
}

// DeleteComment handles DELETE /comments/{id}: its author, or the author of the
// post it sits under, removes it.
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	commentID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid comment id", http.StatusBadRequest)
		return
	}
	if err := h.Service.Delete(userID, commentID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
