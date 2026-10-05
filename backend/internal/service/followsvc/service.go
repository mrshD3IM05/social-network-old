package followsvc

import (
	"errors"
	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/notificationsvc"
)

// how many people the "People you may know" panel shows
const suggestionLimit = 4

var (
	ErrCannotFollowSelf = errors.New("follow: cannot follow yourself")
	ErrExists           = errors.New("follow: relationship already exists")
	ErrNotFound         = errors.New("follow: relationship not found")
	ErrNotRecipient     = errors.New("follow: user is not the recipient")
)

type Service struct {
	repo          *repository.FollowRepository
	users         *repository.UserRepository
	notifications notificationsvc.Notifier
}

func New(repo *repository.FollowRepository, users *repository.UserRepository, notifications notificationsvc.Notifier) *Service {
	return &Service{repo: repo, users: users, notifications: notifications}
}

// Follow sends a follow request. A public profile accepts it right away, a
// private one gets a notification to accept or decline it.
func (s *Service) Follow(from, to int64) (*model.FollowRequest, error) {
	if from == to {
		return nil, ErrCannotFollowSelf
	}
	target, err := s.users.GetUserByID(to)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.GetFollowRequest(from, to)
	status := model.FollowAccepted
	if target.Private {
		status = model.FollowPending
	}
	if err == nil {
		if existing.Status == model.FollowAccepted || existing.Status == model.FollowPending {
			return nil, ErrExists
		}
		if err := s.repo.UpdateFollowStatus(existing.ID, status); err != nil {
			return nil, err
		}
		existing.Status = status
		s.notifyFollow(existing)
		return existing, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	follow, err := s.repo.CreateFollowRequest(from, to, status)
	if err != nil {
		return nil, err
	}
	s.notifyFollow(follow)
	return follow, nil
}

// notifyFollow tells the followed user about a new request or a new follower.
func (s *Service) notifyFollow(follow *model.FollowRequest) {
	name := s.name(follow.FromUserID)
	notification := &model.Notification{UserID: follow.ToUserID, ActorID: follow.FromUserID}
	if follow.Status == model.FollowPending {
		notification.Type = model.NotificationFollowRequest
		notification.Content = name + " wants to follow you"
	} else {
		notification.Type = model.NotificationNewFollower
		notification.Content = name + " started following you"
	}
	s.notifications.Notify(notification)
}

func (s *Service) name(userID int64) string {
	user, err := s.users.GetUserByID(userID)
	if err != nil {
		return "someone"
	}
	return user.FirstName + " " + user.LastName
}
func (s *Service) Unfollow(from, to int64) error { return s.repo.DeleteFollow(from, to) }

func (s *Service) Respond(recipient, requestID int64, status string) error {
	follow, err := s.repo.GetFollowRequestByID(requestID)
	if err != nil {
		return ErrNotFound
	}
	if follow.ToUserID != recipient {
		return ErrNotRecipient
	}
	if follow.Status != model.FollowPending {
		return ErrExists
	}
	if err := s.repo.UpdateFollowStatus(requestID, status); err != nil {
		return err
	}
	if status == model.FollowAccepted {
		s.notifications.Notify(&model.Notification{
			UserID:  follow.FromUserID,
			Type:    model.NotificationFollowAccepted,
			ActorID: recipient,
			Content: s.name(recipient) + " accepted your follow request",
		})
	}
	return nil
}

// AcceptAllPending accepts every request waiting for userID. It runs when a
// profile turns public: a public profile has no requests to answer.
func (s *Service) AcceptAllPending(userID int64) error {
	// the list comes a page at a time; every accepted request leaves it, so
	// the first page is asked for again until nothing is pending
	for {
		requests, err := s.repo.ListPendingFollowRequests(userID, userID, 0)
		if err != nil {
			return err
		}
		if len(requests) == 0 {
			return nil
		}
		for _, request := range requests {
			if err := s.Respond(userID, request.ID, model.FollowAccepted); err != nil {
				return err
			}
		}
	}
}

// PendingRequests are the follow requests waiting for userID to answer, read
// as userID, so each sender carries the relation they have with them.
func (s *Service) PendingRequests(viewerID, userID, lastID int64) ([]*model.FollowRequest, error) {
	return s.repo.ListPendingFollowRequests(viewerID, userID, lastID)
}

// Followers are the users who follow userID, Following the ones userID follows.
// Both only count accepted requests, so a pending one shows up in neither. The
// viewer is who is asking, which is not necessarily userID.
func (s *Service) Followers(viewerID, userID, lastID int64) ([]*model.User, error) {
	return s.repo.ListFollowers(viewerID, userID, lastID)
}
func (s *Service) Following(viewerID, userID, lastID int64) ([]*model.User, error) {
	return s.repo.ListFollowing(viewerID, userID, lastID)
}

// Messageable are the users userID can start a private chat with: at least one
// of the two follows the other, accepted. It is the Messages list, and the same
// rule CanMessage checks before a message goes through.
// Suggestions are a few people the user is not connected to yet.
func (s *Service) Suggestions(userID int64) ([]*model.User, error) {
	return s.repo.ListSuggestedUsers(userID, suggestionLimit)
}

func (s *Service) Messageable(userID, lastID int64) ([]*model.User, error) {
	return s.repo.ListMessageableUsers(userID, lastID)
}
