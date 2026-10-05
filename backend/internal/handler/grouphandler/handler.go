package grouphandler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sn-backend/internal/handler/common"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/eventsvc"
	"sn-backend/internal/service/filesvc"
	"sn-backend/internal/service/groupsvc"
	"sn-backend/internal/service/sessionsvc"
)

type Handler struct {
	Service *groupsvc.Service
	Events  *eventsvc.Service
	File    *filesvc.Service
	Session *sessionsvc.Service
}

func New(service *groupsvc.Service, events *eventsvc.Service, file *filesvc.Service, session *sessionsvc.Service) *Handler {
	return &Handler{Service: service, Events: events, File: file, Session: session}
}

func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	group, err := h.Service.Create(userID, r.FormValue("title"), r.FormValue("description"))
	if err != nil {
		switch {
		case errors.Is(err, groupsvc.ErrInvalidTitle), errors.Is(err, groupsvc.ErrInvalidDescription):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "could not create group", http.StatusInternalServerError)
		}
		return
	}
	common.WriteJSON(w, http.StatusCreated, group)
}

// ListGroups handles GET /groups?joined=&last=: 10 groups at a time, newest
// first, each with its member count and the caller's relation to it.
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	// ?joined=true: the caller's groups, ?joined=false: the others, none: all
	filter := repository.AllGroups
	switch r.URL.Query().Get("joined") {
	case "true":
		filter = repository.JoinedGroups
	case "false":
		filter = repository.OtherGroups
	}
	groups, err := h.Service.List(userID, filter, common.LastID(r))
	if err != nil {
		http.Error(w, "could not list groups", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, http.StatusOK, groups)
}

func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	detail, err := h.Service.Detail(userID, groupID)
	if err != nil {
		writeError(w, err, "could not get group")
		return
	}
	common.WriteJSON(w, http.StatusOK, detail)
}

// UpdateGroup handles PUT /groups/{group_id} (creator only): new title and description.
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	group, err := h.Service.Update(userID, groupID, r.FormValue("title"), r.FormValue("description"))
	if err != nil {
		writeError(w, err, "could not update group")
		return
	}
	common.WriteJSON(w, http.StatusOK, group)
}

// SetGroupAvatar handles POST /groups/{group_id}/avatar (creator only), multipart
// field "avatar", same image rules as the user avatar.
func (h *Handler) SetGroupAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	if err := h.Service.CheckCreator(userID, groupID); err != nil {
		writeError(w, err, "could not set group picture")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, filesvc.MaxImageSize+1<<20)
	if err := r.ParseMultipartForm(filesvc.MaxMemory); err != nil {
		http.Error(w, "upload is too large or invalid", http.StatusBadRequest)
		return
	}
	headers := r.MultipartForm.File["avatar"]
	if len(headers) != 1 {
		http.Error(w, "exactly one image is required", http.StatusBadRequest)
		return
	}
	file, err := h.File.Upload(userID, headers[0], nil, nil, nil)
	if err != nil {
		if filesvc.IsBadImage(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "could not set group picture", http.StatusInternalServerError)
		}
		return
	}
	group, err := h.Service.SetAvatar(userID, groupID, file.ID)
	if err != nil {
		writeError(w, err, "could not set group picture")
		return
	}
	common.WriteJSON(w, http.StatusOK, group)
}

// DeleteGroup handles DELETE /groups/{group_id} (creator only).
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	if err := h.Service.Delete(userID, groupID); err != nil {
		writeError(w, err, "could not delete group")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember handles DELETE /groups/{group_id}/members/{userID}: the creator
// removes a member, or a member removes themselves to leave the group.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	memberID, err := common.PathID(r, "userID")
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.Service.RemoveMember(userID, groupID, memberID); err != nil {
		writeError(w, err, "could not remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMessages handles GET /groups/{group_id}/messages (members only): the chat
// history, new messages then arrive over the websocket.
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	messages, err := h.Service.Messages(userID, groupID, common.LastID(r))
	if err != nil {
		writeError(w, err, "could not list messages")
		return
	}
	common.WriteJSON(w, http.StatusOK, messages)
}

func (h *Handler) GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	members, err := h.Service.Members(userID, groupID, common.LastID(r))
	if err != nil {
		if errors.Is(err, groupsvc.ErrNotGroupMember) {
			http.Error(w, "only group members can view members", http.StatusForbidden)
			return
		}
		writeError(w, err, "could not get group members")
		return
	}
	common.WriteJSON(w, http.StatusOK, members)
}

func (h *Handler) InviteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	toUserID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || toUserID < 1 {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	invitation, err := h.Service.Invite(userID, groupID, toUserID)
	if err != nil {
		writeError(w, err, "could not invite user")
		return
	}
	common.WriteJSON(w, http.StatusCreated, invitation)
}

func (h *Handler) RespondInvitation(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	invitationID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid invitation id", http.StatusBadRequest)
		return
	}
	accept := r.URL.Path == "/group-invitations/"+r.PathValue("id")+"/accept"
	if err := h.Service.RespondInvitation(userID, invitationID, accept); err != nil {
		writeError(w, err, "could not respond to invitation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RequestJoin(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	request, err := h.Service.RequestJoin(userID, groupID)
	if err != nil {
		writeError(w, err, "could not request to join group")
		return
	}
	common.WriteJSON(w, http.StatusCreated, request)
}

func (h *Handler) RespondJoinRequest(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	requestID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid join request id", http.StatusBadRequest)
		return
	}
	accept := r.URL.Path == "/group-join-requests/"+r.PathValue("id")+"/accept"
	if err := h.Service.RespondJoinRequest(userID, requestID, accept); err != nil {
		writeError(w, err, "could not respond to join request")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PendingInvitations(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	invitations, err := h.Service.PendingInvitations(userID, common.LastID(r))
	if err != nil {
		http.Error(w, "could not list pending invitations", http.StatusInternalServerError)
		return
	}
	common.WriteJSON(w, http.StatusOK, invitations)
}

func (h *Handler) PendingJoinRequests(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	requests, err := h.Service.PendingJoinRequests(userID, groupID, common.LastID(r))
	if err != nil {
		if errors.Is(err, groupsvc.ErrNotGroupCreator) {
			http.Error(w, "only the group creator can view join requests", http.StatusForbidden)
			return
		}
		writeError(w, err, "could not list join requests")
		return
	}
	common.WriteJSON(w, http.StatusOK, requests)
}

// ---------------------------------------------------------------- events

// CreateEvent handles POST /groups/{group_id}/events (members only).
func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	event, err := h.Events.Create(userID, groupID, r.FormValue("title"), r.FormValue("description"), parseEventDateTime(r.FormValue("date"), r.FormValue("time")))
	if err != nil {
		writeEventError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusCreated, event)
}

// ListEvents handles GET /groups/{group_id}/events (members only).
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	groupID, err := common.PathID(r, "group_id")
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	events, err := h.Events.List(userID, groupID, common.LastID(r))
	if err != nil {
		writeEventError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusOK, events)
}

// UpcomingEvents handles GET /events/upcoming: the next events across all of
// the viewer's groups, in one request.
func (h *Handler) UpcomingEvents(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	events, err := h.Events.Upcoming(userID)
	if err != nil {
		writeEventError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusOK, events)
}

// RespondEvent handles POST /events/{id}/response (group members only).
func (h *Handler) RespondEvent(w http.ResponseWriter, r *http.Request) {
	userID, err := common.CurrentUserID(r, h.Session)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	eventID, err := common.PathID(r, "id")
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	choice := strings.TrimSpace(r.FormValue("choice"))
	going, notGoing, err := h.Events.Respond(userID, eventID, choice)
	if err != nil {
		writeEventError(w, err)
		return
	}
	common.WriteJSON(w, http.StatusOK, map[string]any{
		"event_id":        eventID,
		"my_choice":       choice,
		"going_count":     going,
		"not_going_count": notGoing,
	})
}

// parseEventDateTime combines the HTML date/time inputs (date "2006-01-02",
// time "15:04") into a time.Time. The zero time signals "missing", which the
// service rejects.
func parseEventDateTime(date, clock string) time.Time {
	if date == "" || clock == "" {
		return time.Time{}
	}
	value, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, time.Local)
	if err != nil {
		return time.Time{}
	}
	return value
}

func writeEventError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, eventsvc.ErrInvalidTitle),
		errors.Is(err, eventsvc.ErrInvalidDescription),
		errors.Is(err, eventsvc.ErrInvalidDateTime),
		errors.Is(err, eventsvc.ErrInvalidChoice):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, eventsvc.ErrNotFound),
		errors.Is(err, repository.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, eventsvc.ErrNotGroupMember):
		http.Error(w, "only group members can do that", http.StatusForbidden)
	default:
		http.Error(w, "could not process event", http.StatusInternalServerError)
	}
}

// writeError maps service and repository errors to the project's flat-text
// http.Error responses with the same status-code vocabulary as the other
// handlers.
func writeError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, groupsvc.ErrInvalidTitle),
		errors.Is(err, groupsvc.ErrInvalidDescription),
		errors.Is(err, groupsvc.ErrSelfInvite),
		errors.Is(err, groupsvc.ErrSelfRequest),
		errors.Is(err, groupsvc.ErrRemoveCreator):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, groupsvc.ErrNotFound),
		errors.Is(err, repository.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, groupsvc.ErrAlreadyMember),
		errors.Is(err, groupsvc.ErrInvitationExists),
		errors.Is(err, groupsvc.ErrRequestExists),
		errors.Is(err, repository.ErrExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, groupsvc.ErrNotGroupMember),
		errors.Is(err, groupsvc.ErrNotGroupCreator),
		errors.Is(err, groupsvc.ErrNotRecipient),
		errors.Is(err, repository.ErrNotOwner):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}
