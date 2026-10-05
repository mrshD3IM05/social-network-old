package usersvc

import (
	"errors"
	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"strings"
)

type Service struct {
	users   *repository.UserRepository
	follows *repository.FollowRepository
}

func New(users *repository.UserRepository, follows *repository.FollowRepository) *Service {
	return &Service{users: users, follows: follows}
}

// GetUser reads one user as the viewer may see them, with the follow counts
// and the relation the two of them have.
func (s *Service) GetUser(viewerID, id int64) (*model.User, error) {
	return s.users.GetUserForViewer(viewerID, id)
}

// ListUsers is the directory behind GET /users: every registered user except
// the viewer. Private profiles stay in the list — common.Profile decides what
// each row may show about itself.
func (s *Service) ListUsers(viewerID int64, search string, lastID int64) ([]*model.User, error) {
	return s.users.ListUsers(viewerID, strings.TrimSpace(search), lastID)
}

// CanViewProfile is whether viewerID may read the content of a profile: a
// private one only opens up to its followers. The profile itself is always
// answered — this gate is for the posts behind it.
func (s *Service) CanViewProfile(viewerID int64, user *model.User) (bool, error) {
	if !user.Private || viewerID == user.ID {
		return true, nil
	}
	if viewerID == 0 {
		return false, nil
	}
	return s.follows.IsFollowing(viewerID, user.ID)
}

// SetPrivacy turns the caller's own profile public or private and answers with
// the stored user, so the client never has to guess what was saved.
func (s *Service) SetPrivacy(userID int64, private bool) (*model.User, error) {
	if err := s.users.SetUserPrivate(userID, private); err != nil {
		return nil, err
	}
	return s.users.GetUserByID(userID)
}

func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }
