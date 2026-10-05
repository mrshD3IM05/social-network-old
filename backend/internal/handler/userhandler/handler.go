package userhandler

import (
	"net/http"
	"sn-backend/internal/handler/common"
	"sn-backend/internal/model"
	"sn-backend/internal/service/followsvc"
	"sn-backend/internal/service/postsvc"
	"sn-backend/internal/service/sessionsvc"
	"sn-backend/internal/service/usersvc"
	"strconv"
)

type Handler struct {
	Service *usersvc.Service
	Session *sessionsvc.Service
	Follow  *followsvc.Service
	Post    *postsvc.Service
}

func New(service *usersvc.Service, session *sessionsvc.Service, follow *followsvc.Service, post *postsvc.Service) *Handler {
	return &Handler{Service: service, Session: session, Follow: follow, Post: post}
}

// ListUsers handles GET /users?q=&last=: one page of the people directory
// every "pick a person" screen reads from (People, Messages, group invites),
// searched by name or nickname. It never includes the caller and only exposes
// the public profile fields, plus the follow counts and the relation the caller
// has with each row.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	viewerID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	users, err := h.Service.ListUsers(viewerID, r.URL.Query().Get("q"), common.LastID(r))
	if err != nil {
		http.Error(w, "could not list users", http.StatusInternalServerError)
		return
	}
	writePeople(w, users)
}

// GetUser handles GET /user/{id}: the profile page, always answered. A private
// profile is not a 403 here — common.Profile leaves out the contact details the
// caller is not entitled to, and the client reads what it may show off the
// private and is_following fields.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, viewerID, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	profile := common.Profile(user, viewerID)
	common.WriteJSON(w, http.StatusOK, profile)
}

// UserPosts handles GET /users/{id}/posts?last=: one page of the posts on a
// profile, behind the privacy gate that is left for the content itself — the
// profile above opens up to everyone.
func (h *Handler) UserPosts(w http.ResponseWriter, r *http.Request) {
	user, viewerID, ok := h.visibleUser(w, r)
	if !ok {
		return
	}
	posts, err := h.Post.ListPosts(viewerID, user.ID, nil, common.LastID(r))
	if err != nil {
		http.Error(w, "could not list posts", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, http.StatusOK, posts)
}

// SetPrivacy handles PUT /me/privacy: the switch on your own profile that turns
// it public or private. It always acts on the caller, so one user can never
// change another user's privacy.
func (h *Handler) SetPrivacy(w http.ResponseWriter, r *http.Request) {
	viewerID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	private, err := strconv.ParseBool(r.FormValue("private"))
	if err != nil {
		http.Error(w, "private must be true or false", http.StatusBadRequest)
		return
	}
	user, err := h.Service.SetPrivacy(viewerID, private)
	if err != nil {
		http.Error(w, "could not update profile privacy", http.StatusInternalServerError)
		return
	}
	// a public profile is followed without asking, so the waiting requests
	// are accepted at once
	if !private {
		if err := h.Follow.AcceptAllPending(viewerID); err != nil {
			http.Error(w, "could not accept pending follow requests", http.StatusInternalServerError)
			return
		}
	}
	common.WriteJSON(w, http.StatusOK, common.PrivateUser(user))
}

// resolveUser resolves the {id} in the path, reads the caller out of the cookie
// and loads that profile with the relation the two of them have. It writes the
// error itself and answers false once the caller should stop. A private profile
// is no obstacle here: the profile is answered to everyone, and common.Profile
// is what holds the contact details back.
// The cookie is read leniently, so a caller with no usable session reads the
// profile as anonymous — the relations then come back 0 — rather than being
// turned away.
func (h *Handler) resolveUser(w http.ResponseWriter, r *http.Request) (*model.User, int64, bool) {
	id, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return nil, 0, false
	}
	viewerID := int64(0)
	if cookie, cookieErr := r.Cookie(sessionsvc.CookieName); cookieErr == nil {
		if session, sessionErr := h.Session.Get(cookie.Value); sessionErr == nil {
			viewerID = session.UserID
		}
	}
	user, err := h.Service.GetUser(viewerID, id)
	if err != nil {
		if usersvc.IsNotFound(err) {
			http.Error(w, "user not found", http.StatusNotFound)
		} else {
			http.Error(w, "could not get user", http.StatusInternalServerError)
		}
		return nil, 0, false
	}
	return user, viewerID, true
}

// visibleUser is resolveUser plus the gate left on the content behind a private
// profile: only its followers may read it. The profile itself does not go
// through here.
func (h *Handler) visibleUser(w http.ResponseWriter, r *http.Request) (*model.User, int64, bool) {
	user, viewerID, ok := h.resolveUser(w, r)
	if !ok {
		return nil, 0, false
	}
	visible, err := h.Service.CanViewProfile(viewerID, user)
	if err != nil {
		http.Error(w, "could not check profile access", http.StatusInternalServerError)
		return nil, 0, false
	}
	if !visible {
		http.Error(w, "profile is private", http.StatusForbidden)
		return nil, 0, false
	}
	return user, viewerID, true
}

// writePeople answers with the public profile of every user in the list.
func writePeople(w http.ResponseWriter, users []*model.User) {
	people := make([]map[string]any, 0, len(users))
	for _, user := range users {
		people = append(people, common.PublicUser(user))
	}
	common.WriteJSON(w, http.StatusOK, people)
}
