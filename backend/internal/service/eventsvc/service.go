package eventsvc

import (
	"errors"
	"log"
	"strings"
	"time"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/notificationsvc"
)

const (
	maxTitleLen       = 100
	maxDescriptionLen = 1000
	upcomingLimit     = 3
)

var (
	ErrInvalidTitle       = errors.New("event: title is required (max 100 chars)")
	ErrInvalidDescription = errors.New("event: description is too long (max 1000 chars)")
	ErrInvalidDateTime    = errors.New("event: a valid future date and time is required")
	ErrNotFound           = errors.New("event: not found")
	ErrInvalidChoice      = errors.New("event: choice must be going or not_going")
	ErrNotGroupMember     = errors.New("event: only group members can do that")
)

// Service uses the hub to notify group members of new events.
type Service struct {
	repo          *repository.EventRepository
	groups        *repository.GroupRepository
	users         *repository.UserRepository
	notifications notificationsvc.Notifier
}

func New(repo *repository.EventRepository, groups *repository.GroupRepository, users *repository.UserRepository, notifications notificationsvc.Notifier) *Service {
	return &Service{repo: repo, groups: groups, users: users, notifications: notifications}
}

// Create adds an event to a group. Every member may create events: the
// subject gives no special event role, and this matches how posting in the
// group works.
func (s *Service) Create(creatorID, groupID int64, title, description string, dateTime time.Time) (*model.GroupEvent, error) {
	if _, err := s.groups.GetGroup(groupID); err != nil {
		return nil, err // repository.ErrNotFound → 404
	}
	member, err := s.groups.IsGroupMember(groupID, creatorID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotGroupMember
	}

	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" || len(title) > maxTitleLen {
		return nil, ErrInvalidTitle
	}
	if len(description) > maxDescriptionLen {
		return nil, ErrInvalidDescription
	}
	if dateTime.IsZero() || dateTime.Before(time.Now().Add(-time.Minute)) {
		return nil, ErrInvalidDateTime
	}

	event := &model.GroupEvent{
		GroupID:     groupID,
		CreatorID:   creatorID,
		Title:       title,
		Description: description,
		DateTime:    dateTime,
	}
	if err := s.repo.CreateEvent(event); err != nil {
		return nil, err
	}

	s.notifyMembers(event)
	return event, nil
}

// List returns the events of a group with counts and the viewer's choice.
// Members only.
func (s *Service) List(viewerID, groupID, lastID int64) ([]*model.EventListItem, error) {
	member, err := s.groups.IsGroupMember(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotGroupMember
	}
	return s.repo.ListGroupEvents(groupID, viewerID, lastID)
}

// Upcoming returns the next few events across all of the viewer's groups.
// Membership is enforced by the query itself.
func (s *Service) Upcoming(viewerID int64) ([]*model.UpcomingEvent, error) {
	return s.repo.ListUpcomingEvents(viewerID, time.Now(), upcomingLimit)
}

// Respond sets or changes the viewer's going / not-going answer. One row per
// (event, user) is guaranteed by the event_responses UNIQUE constraint and
// the upsert, so changing the answer replaces the old one.
// Respond saves the viewer's answer to an event. An empty choice removes it,
// so a member can take back a "going" or "not going".
func (s *Service) Respond(viewerID, eventID int64, choice string) (going, notGoing int, err error) {
	if choice != "" && choice != model.EventChoiceGoing && choice != model.EventChoiceNotGoing {
		return 0, 0, ErrInvalidChoice
	}
	groupID, err := s.repo.GetGroupIDForEvent(eventID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	member, err := s.groups.IsGroupMember(groupID, viewerID)
	if err != nil {
		return 0, 0, err
	}
	if !member {
		return 0, 0, ErrNotGroupMember
	}

	if choice == "" {
		err = s.repo.DeleteEventResponse(eventID, viewerID)
	} else {
		err = s.repo.SetEventResponse(eventID, viewerID, choice)
	}
	if err != nil {
		return 0, 0, err
	}
	going, notGoing, err = s.repo.EventResponseCounts(eventID)
	return going, notGoing, err
}

// notifyMembers tells every member except the creator that a new event was
// scheduled (subject requirement). Non-members never receive it because the
// recipients come straight from group_members.
func (s *Service) notifyMembers(event *model.GroupEvent) {
	memberIDs, err := s.groups.GroupMemberIDs(event.GroupID)
	if err != nil {
		log.Printf("eventsvc: could not list members for notification: %v", err)
		return
	}
	group, err := s.groups.GetGroup(event.GroupID)
	if err != nil {
		return
	}
	creator, err := s.users.GetUserByID(event.CreatorID)
	if err != nil {
		return
	}
	name := strings.TrimSpace(creator.FirstName + " " + creator.LastName)
	if name == "" {
		name = creator.Nickname
	}
	for _, memberID := range memberIDs {
		if memberID == event.CreatorID {
			continue
		}
		s.notifications.Notify(&model.Notification{
			UserID:  memberID,
			Type:    model.NotificationEventCreated,
			ActorID: event.CreatorID,
			Content: name + " created the event \"" + event.Title + "\" in \"" + group.Title + "\"",
			GroupID: &event.GroupID,
		})
	}
}
