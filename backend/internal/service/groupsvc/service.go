package groupsvc

import (
	"errors"
	"strings"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/notificationsvc"
)

const (
	maxTitleLen       = 100
	maxDescriptionLen = 1000
)

var (
	ErrInvalidTitle       = errors.New("group: title is required (max 100 chars)")
	ErrInvalidDescription = errors.New("group: description is too long (max 1000 chars)")
	ErrNotFound           = errors.New("group: not found")
	ErrAlreadyMember      = errors.New("group: user is already a member")
	ErrInvitationExists   = errors.New("group: invitation already pending")
	ErrRequestExists      = errors.New("group: join request already pending")
	ErrNotGroupMember     = errors.New("group: only group members can do that")
	ErrNotGroupCreator    = errors.New("group: only the group creator can do that")
	ErrNotRecipient       = errors.New("group: user is not the invitation recipient")
	ErrSelfInvite         = errors.New("group: cannot invite yourself")
	ErrSelfRequest        = errors.New("group: cannot request to join your own group")
	ErrRemoveCreator      = errors.New("group: the creator cannot be removed")
)

type Service struct {
	repo          *repository.GroupRepository
	users         *repository.UserRepository
	events        *repository.EventRepository
	messages      *repository.MessageRepository
	notifications notificationsvc.Notifier
}

func New(repo *repository.GroupRepository, users *repository.UserRepository, events *repository.EventRepository, messages *repository.MessageRepository, notifications notificationsvc.Notifier) *Service {
	return &Service{repo: repo, users: users, events: events, messages: messages, notifications: notifications}
}

func (s *Service) Create(creatorID int64, title, description string) (*model.Group, error) {
	title, description, err := checkGroupInfo(title, description)
	if err != nil {
		return nil, err
	}

	group := &model.Group{CreatorID: creatorID, Title: title, Description: description}
	if err := s.repo.CreateGroup(group); err != nil {
		return nil, err
	}
	// The creator is a member from the start.
	if err := s.repo.AddGroupMember(group.ID, creatorID); err != nil {
		return nil, err
	}
	return group, nil
}

// Update changes the title and description. Only the creator can do it.
func (s *Service) Update(creatorID, groupID int64, title, description string) (*model.Group, error) {
	group, err := s.creatorGroup(creatorID, groupID)
	if err != nil {
		return nil, err
	}
	group.Title, group.Description, err = checkGroupInfo(title, description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateGroup(group); err != nil {
		return nil, err
	}
	return group, nil
}

// SetAvatar stores an uploaded file id as the group picture (creator only).
func (s *Service) SetAvatar(creatorID, groupID int64, fileID string) (*model.Group, error) {
	group, err := s.creatorGroup(creatorID, groupID)
	if err != nil {
		return nil, err
	}
	group.Avatar = fileID
	if err := s.repo.UpdateGroup(group); err != nil {
		return nil, err
	}
	return group, nil
}

// Delete removes the group and, through the database cascade, everything in
// it. Only the creator can do it.
func (s *Service) Delete(creatorID, groupID int64) error {
	if _, err := s.creatorGroup(creatorID, groupID); err != nil {
		return err
	}
	return s.repo.DeleteGroup(groupID)
}

// RemoveMember lets the creator kick a member out of the group, or a member
// leave it by removing themselves. The creator can never be removed: they
// delete the group instead.
func (s *Service) RemoveMember(viewerID, groupID, userID int64) error {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return err
	}
	if userID == group.CreatorID {
		return ErrRemoveCreator
	}
	leaving := viewerID == userID
	if !leaving && viewerID != group.CreatorID {
		return ErrNotGroupCreator
	}
	isMember, err := s.repo.IsGroupMember(groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotFound
	}
	if err := s.repo.RemoveGroupMember(groupID, userID); err != nil {
		return err
	}
	if leaving {
		return nil
	}
	s.notifications.Notify(&model.Notification{
		UserID:  userID,
		Type:    model.NotificationGroupRemoved,
		ActorID: viewerID,
		Content: "you were removed from \"" + group.Title + "\"",
		GroupID: &group.ID,
	})
	return nil
}

// Messages returns the group chat history, members only.
func (s *Service) Messages(viewerID, groupID, lastID int64) ([]*model.Message, error) {
	isMember, err := s.repo.IsGroupMember(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotGroupMember
	}
	return s.messages.ListGroupMessages(groupID, lastID)
}

// CheckCreator reports ErrNotGroupCreator (or ErrNotFound) when userID does
// not own the group; the handler calls it before storing an upload.
func (s *Service) CheckCreator(userID, groupID int64) error {
	_, err := s.creatorGroup(userID, groupID)
	return err
}

// List returns one page of groups (newest first) after lastID, narrowed by
// filter to the viewer's groups, the others, or all of them.
func (s *Service) List(viewerID int64, filter repository.GroupFilter, lastID int64) ([]*model.GroupListItem, error) {
	return s.repo.GroupListPayload(viewerID, filter, lastID)
}

// Detail returns one group for a viewer. Every group is listed on the groups
// page, so outsiders can open it too, but they only see the header (title,
// description, creator, member count): the member list stays members only.
func (s *Service) Detail(viewerID, groupID int64) (*model.GroupDetail, error) {
	detail, err := s.repo.GroupDetailPayload(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !detail.IsMember && !detail.IsCreator {
		detail.Members = []model.GroupMember{}
		pendingInvitation, err := s.repo.PendingGroupInvitation(groupID, viewerID)
		if err == nil {
			detail.PendingInvite = pendingInvitation.Status == model.GroupInvitationPending
			if detail.PendingInvite {
				detail.InvitationID = pendingInvitation.ID
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	} else {
		// events are members only, and come 10 at a time: the tab count comes from here
		detail.EventCount, err = s.events.CountGroupEvents(groupID)
		if err != nil {
			return nil, err
		}
	}
	return detail, nil
}

// Members are visible to group members only.
func (s *Service) Members(viewerID, groupID, lastID int64) ([]*model.GroupMember, error) {
	member, err := s.repo.IsGroupMember(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotGroupMember
	}
	return s.repo.GetGroupMembers(groupID, lastID)
}

// Invite lets a current member invite an existing user. It rejects self
// invites, unknown users, existing members, duplicate pending invitations
// and stale/processed ones being reused.
func (s *Service) Invite(memberID, groupID, toUserID int64) (*model.GroupInvitation, error) {
	if memberID == toUserID {
		return nil, ErrSelfInvite
	}
	isMember, err := s.repo.IsGroupMember(groupID, memberID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotGroupMember
	}
	if _, err := s.users.GetUserByID(toUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound // target user does not exist
		}
		return nil, err
	}
	targetIsMember, err := s.repo.IsGroupMember(groupID, toUserID)
	if err != nil {
		return nil, err
	}
	if targetIsMember {
		return nil, ErrAlreadyMember
	}

	existing, err := s.repo.PendingGroupInvitation(groupID, toUserID)
	switch {
	case err == nil:
		if existing.Status == model.GroupInvitationPending {
			return nil, ErrInvitationExists
		}
		// The schema keeps one row per (group, user): drop the old answered
		// invitation so the user can be invited again.
		if err := s.repo.DeleteGroupInvitation(existing.ID); err != nil {
			return nil, err
		}
	case errors.Is(err, repository.ErrNotFound):
		// no previous invitation for this (group, user) pair
	default:
		return nil, err
	}

	invitation := &model.GroupInvitation{
		GroupID:    groupID,
		FromUserID: memberID,
		ToUserID:   toUserID,
		Status:     model.GroupInvitationPending,
	}
	created, err := s.repo.CreateGroupInvitation(invitation)
	if err != nil {
		if isUnique(err) {
			return nil, ErrInvitationExists
		}
		return nil, err
	}

	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	inviter, err := s.users.GetUserByID(memberID)
	if err != nil {
		return nil, err
	}
	notification := &model.Notification{
		UserID:  toUserID,
		Type:    model.NotificationGroupInvite,
		ActorID: memberID,
		Content: inviter.FirstName + " " + inviter.LastName + " invited you to join \"" + group.Title + "\"",
		GroupID: &group.ID,
	}
	s.notifications.Notify(notification)
	return created, nil
}

// RespondInvitation accepts or refuses an invitation. Only the recipient of
// the pending invitation may respond to it.
func (s *Service) RespondInvitation(userID, invitationID int64, accept bool) error {
	if accept {
		if err := s.repo.AcceptGroupInvitationTx(invitationID, userID); err != nil {
			return err
		}
		invitation, err := s.repo.GetGroupInvitationByID(invitationID)
		if err != nil {
			return err
		}
		name := "someone"
		if user, err := s.users.GetUserByID(userID); err == nil {
			name = user.FirstName + " " + user.LastName
		}
		notification := &model.Notification{
			UserID:  invitation.FromUserID,
			Type:    model.NotificationGroupInviteResp,
			ActorID: userID,
			Content: name + " accepted your invitation to \"" + invitation.GroupTitle + "\"",
			GroupID: &invitation.GroupID,
		}
		s.notifications.Notify(notification)
		return nil
	}
	return s.repo.RefuseGroupInvitationTx(invitationID, userID)
}

// RequestJoin lets a non-member ask to join a group. Creators do not request
// to join their own group and duplicates are rejected.
func (s *Service) RequestJoin(userID, groupID int64) (*model.GroupJoinRequest, error) {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	if group.CreatorID == userID {
		return nil, ErrSelfRequest
	}
	isMember, err := s.repo.IsGroupMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if isMember {
		return nil, ErrAlreadyMember
	}

	existing, err := s.repo.PendingGroupJoinRequest(groupID, userID)
	switch {
	case err == nil:
		if existing.Status == model.GroupJoinPending {
			return nil, ErrRequestExists
		}
		// Same as invitations: drop the old answered request first.
		if err := s.repo.DeleteGroupJoinRequest(existing.ID); err != nil {
			return nil, err
		}
	case errors.Is(err, repository.ErrNotFound):
		// no previous request for this (group, user) pair
	default:
		return nil, err
	}

	request := &model.GroupJoinRequest{
		GroupID: groupID,
		UserID:  userID,
		Status:  model.GroupJoinPending,
	}
	created, err := s.repo.CreateGroupJoinRequest(request)
	if err != nil {
		if isUnique(err) {
			return nil, ErrRequestExists
		}
		return nil, err
	}

	notification := &model.Notification{
		UserID:  group.CreatorID,
		Type:    model.NotificationGroupJoinReq,
		ActorID: userID,
		Content: requesterName(created) + " requested to join \"" + group.Title + "\"",
		GroupID: &group.ID,
	}
	s.notifications.Notify(notification)
	return created, nil
}

// CancelJoinRequest lets a user cancel their own pending join request.
func (s *Service) CancelJoinRequest(userID, groupID int64) error {
	request, err := s.repo.PendingGroupJoinRequest(groupID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	// PendingGroupJoinRequest answers regardless of status, and answering a
	// request updates the row instead of removing it. Only a still-pending one
	// can be withdrawn: an accepted request must stay put, or the user would
	// keep their membership while the record of the request disappeared.
	if request.Status != model.GroupJoinPending {
		return ErrNotFound
	}
	return s.repo.DeleteGroupJoinRequest(request.ID)
}

// RespondJoinRequest accepts or refuses a join request. Only the group
// creator may respond to requests for their group.
func (s *Service) RespondJoinRequest(creatorID, requestID int64, accept bool) error {
	request, err := s.repo.GetGroupJoinRequestByID(requestID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	group, err := s.repo.GetGroup(request.GroupID)
	if err != nil {
		return err
	}
	if group.CreatorID != creatorID {
		return ErrNotGroupCreator
	}

	if !accept {
		return s.repo.RefuseGroupJoinRequestTx(requestID, request.GroupID)
	}

	if err := s.repo.AcceptGroupJoinRequestTx(requestID, request.GroupID); err != nil {
		return err
	}
	notification := &model.Notification{
		UserID:  request.UserID,
		Type:    model.NotificationGroupJoinResp,
		ActorID: creatorID,
		Content: "your request to join \"" + group.Title + "\" was accepted",
		GroupID: &group.ID,
	}
	s.notifications.Notify(notification)
	return nil
}

func (s *Service) PendingInvitations(userID, lastID int64) ([]*model.GroupInvitation, error) {
	return s.repo.GetPendingInvitationsForUser(userID, lastID)
}

func (s *Service) PendingJoinRequests(viewerID, groupID, lastID int64) ([]*model.GroupJoinRequest, error) {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	if group.CreatorID != viewerID {
		return nil, ErrNotGroupCreator
	}
	return s.repo.GetPendingJoinRequestsForGroup(groupID, lastID)
}

// MyJoinRequests are the pending join requests of every group userID created.
func (s *Service) MyJoinRequests(userID, lastID int64) ([]*model.GroupJoinRequest, error) {
	return s.repo.GetPendingJoinRequestsForCreator(userID, lastID)
}

// creatorGroup loads the group and checks userID is its creator.
func (s *Service) creatorGroup(userID, groupID int64) (*model.Group, error) {
	group, err := s.repo.GetGroup(groupID)
	if err != nil {
		return nil, err
	}
	if group.CreatorID != userID {
		return nil, ErrNotGroupCreator
	}
	return group, nil
}

func checkGroupInfo(title, description string) (string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" || len(title) > maxTitleLen {
		return "", "", ErrInvalidTitle
	}
	if len(description) > maxDescriptionLen {
		return "", "", ErrInvalidDescription
	}
	return title, description, nil
}

func requesterName(request *model.GroupJoinRequest) string {
	name := strings.TrimSpace(request.FirstName + " " + request.LastName)
	if name == "" {
		name = request.Nickname
	}
	return name
}

// isUnique reports SQLite UNIQUE-constraint violations from the driver so
// races between the pre-check and the insert stay safe.
func isUnique(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
