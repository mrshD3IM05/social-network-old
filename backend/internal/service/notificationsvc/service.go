package notificationsvc

import (
	"log"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
)

type Publisher interface {
	PublishNotification(*model.Notification)
}

type Notifier interface {
	Notify(*model.Notification)
}

type Service struct {
	repo      *repository.NotificationRepository
	publisher Publisher
}

func New(repo *repository.NotificationRepository, publisher Publisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

// Notify persists the event before publishing it. Notification delivery is
// best-effort and must not fail the feature action that caused it.
func (s *Service) Notify(notification *model.Notification) {
	if notification == nil {
		return
	}
	if err := s.repo.CreateNotification(notification); err != nil {
		log.Printf("notificationsvc: could not create notification: %v", err)
		return
	}
	if s.publisher != nil {
		s.publisher.PublishNotification(notification)
	}
}

func (s *Service) List(userID, lastID int64) ([]*model.Notification, error) {
	return s.repo.ListNotifications(userID, lastID)
}

func (s *Service) UnreadCount(userID int64) (int, error) {
	return s.repo.CountUnreadNotifications(userID)
}

func (s *Service) MarkRead(userID int64) error {
	return s.repo.MarkNotificationsRead(userID)
}
