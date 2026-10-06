package posthandler

import (
	"errors"
	"net/http"
	"sn-backend/internal/handler/common"
	"sn-backend/internal/model"
	"sn-backend/internal/service/filesvc"
	"sn-backend/internal/service/postsvc"
	"sn-backend/internal/service/sessionsvc"
	"strconv"
	"strings"
)

type Handler struct {
	Service *postsvc.Service
	Viewers *postsvc.ViewerService
	Files   *filesvc.Service
	Session *sessionsvc.Service
}

func New(service *postsvc.Service, viewers *postsvc.ViewerService, files *filesvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Viewers: viewers, Files: files, Session: session}
}
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
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
	groupID, groupScoped, err := groupScope(r)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	privacy := r.FormValue("privacy")
	var viewers []int64
	if groupScoped {
		privacy = model.PostPublic
	} else {
		viewers, err = formIDs(r, "viewers")
		if err != nil {
			http.Error(w, "invalid viewers", http.StatusBadRequest)
			return
		}
	}
	post, err := h.Service.CreatePost(userID, groupID, r.FormValue("content"), privacy, viewers, len(headers) > 0)
	if err != nil {
		writePostError(w, err, "could not create post")
		return
	}
	if len(headers) > 0 {
		stored, err := h.Files.UploadMany(userID, headers, &post.ID, nil, nil)
		if err != nil {
			_ = h.Service.DeletePost(userID, post.ID, groupID)
			status := http.StatusInternalServerError
			if filesvc.IsBadImage(err) {
				status = http.StatusBadRequest
			}
			http.Error(w, "could not store post images", status)
			return
		}
		for _, file := range stored {
			post.Images = append(post.Images, file.ID)
		}
	}
	common.WriteJSON(w, http.StatusCreated, post)
}
func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	viewerID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, _, err := groupScope(r)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	posts, err := h.Service.ListPosts(viewerID, 0, groupID, common.LastID(r))
	if err != nil {
		writePostError(w, err, "could not list posts")
		return
	}
	common.WriteJSON(w, http.StatusOK, posts)
}

// GetPost handles GET /posts/{id}. Visibility follows CanViewPost: privacy
// rules for normal posts, group membership for group posts â€” invisible posts
// answer 404 like the reaction endpoints.
func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	viewerID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	id, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	post, err := h.Service.GetPost(viewerID, id)
	if err != nil {
		if errors.Is(err, postsvc.ErrNotFound) {
			http.Error(w, "post not found", http.StatusNotFound)
		} else {
			http.Error(w, "could not get post", http.StatusInternalServerError)
		}
		return
	}
	common.WriteJSON(w, http.StatusOK, post)
}

// ListViewers handles GET /posts/{id}/viewers: the followers a private post was
// shared with. Only the author gets an answer.
func (h *Handler) ListViewers(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	id, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	viewers, err := h.Viewers.Viewers(userID, id)
	if err != nil {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}
	common.WriteJSON(w, http.StatusOK, viewers)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, postIDField, err := updatePostPath(r)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	id, err := common.PathID(r, postIDField)
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	viewers, err := formIDs(r, "viewers")
	if err != nil {
		http.Error(w, "invalid viewers", http.StatusBadRequest)
		return
	}
	attachments := formAttachments(r)
	post, err := h.Service.UpdatePost(userID, id, groupID, r.FormValue("content"), r.FormValue("privacy"), viewers, attachments)
	if err != nil {
		if errors.Is(err, postsvc.ErrInvalidPrivacy) || errors.Is(err, postsvc.ErrInvalidContent) || errors.Is(err, postsvc.ErrInvalidViewers) || errors.Is(err, postsvc.ErrInvalidFiles) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if errors.Is(err, postsvc.ErrNotFound) {
			http.Error(w, "post not found", http.StatusNotFound)
		} else if errors.Is(err, postsvc.ErrNotGroupMember) || errors.Is(err, postsvc.ErrForbidden) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "could not update post", http.StatusInternalServerError)
		}
		return
	}
	common.WriteJSON(w, http.StatusOK, post)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	var groupID *int64
	postIDField := "id"
	if r.PathValue("post_id") != "" {
		var groupScoped bool
		groupID, groupScoped, err = groupScope(r)
		if err != nil {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}
		if !groupScoped {
			http.Error(w, "invalid group id", http.StatusBadRequest)
			return
		}
		postIDField = "post_id"
	}
	postID, err := common.PathID(r, postIDField)
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	if err := h.Service.DeletePost(userID, postID, groupID); err != nil {
		writePostError(w, err, "could not delete post")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func groupScope(r *http.Request) (*int64, bool, error) {
	value := r.PathValue("group_id")
	if value == "" {
		return nil, false, nil
	}
	groupID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || groupID < 1 {
		return nil, true, strconv.ErrSyntax
	}
	return &groupID, true, nil
}

func updatePostPath(r *http.Request) (*int64, string, error) {
	if r.PathValue("post_id") == "" {
		return nil, "id", nil
	}
	groupID, groupScoped, err := groupScope(r)
	if err != nil {
		return nil, "", err
	}
	if !groupScoped {
		return nil, "", strconv.ErrSyntax
	}
	return groupID, "post_id", nil
}

func writePostError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, postsvc.ErrInvalidPrivacy), errors.Is(err, postsvc.ErrInvalidContent), errors.Is(err, postsvc.ErrInvalidViewers):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, postsvc.ErrNotFound):
		http.Error(w, "post not found", http.StatusNotFound)
	case errors.Is(err, postsvc.ErrNotGroupMember), errors.Is(err, postsvc.ErrForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

// formIDs reads a list of user ids sent as the same form field repeated
// (viewers=2&viewers=5).
func formIDs(r *http.Request, name string) ([]int64, error) {
	ids := []int64{}
	for _, value := range r.Form[name] {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			return nil, strconv.ErrSyntax
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func formAttachments(r *http.Request) []string {
	values, exists := r.Form["attachments"]
	if !exists {
		return nil
	}
	ids := make([]string, 0, len(values))
	for _, value := range values {
		if id := strings.TrimSpace(value); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
