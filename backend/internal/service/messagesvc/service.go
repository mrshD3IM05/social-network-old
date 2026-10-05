package messagesvc

import (
	"errors"
	"strings"
	"unicode/utf8"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
)

const (
	MaxContentLength = 1000
	DefaultLimit     = repository.MessagePageSize
	MaxLimit         = 200
)

var (
	ErrNotAllowed = errors.New("message: you cannot write here")
	ErrEmpty      = errors.New("message: write something or add an image")
	ErrTooLong    = errors.New("message: content is too long")
)

type Service struct{ repo *repository.MessageRepository }

func New(repo *repository.MessageRepository) *Service { return &Service{repo: repo} }

// History returns the stored conversation with one user. The same rule the
// websocket applies before accepting a message guards it, so history cannot be
// read by someone who could not have taken part in it.
func (s *Service) History(viewerID, otherID int64, limit int, lastID int64) ([]*model.Message, error) {
	allowed, err := s.repo.CanMessage(viewerID, &otherID, nil)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotAllowed
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}
	return s.repo.ListMessages(viewerID, otherID, limit, lastID)
}

// Send saves a message, either to one person or to a group chat.
// withImages says whether pictures will be attached afterwards, which is what
// allows a message with no text.
func (s *Service) Send(fromID int64, toUserID, groupID *int64, content string, withImages bool) (*model.Message, error) {
	allowed, err := s.repo.CanMessage(fromID, toUserID, groupID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotAllowed
	}

	content = strings.TrimSpace(content)
	if content == "" && !withImages {
		return nil, ErrEmpty
	}
	if utf8.RuneCountInString(content) > MaxContentLength {
		return nil, ErrTooLong
	}

	message := &model.Message{
		FromUserID: fromID,
		ToUserID:   toUserID,
		GroupID:    groupID,
		Content:    content,
		Images:     []string{},
	}
	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}
	return message, nil
}

// LoadImages fills in the pictures of a message once they are uploaded.
func (s *Service) LoadImages(message *model.Message) error {
	images, err := s.repo.ListMessageFileIDs(message.ID)
	if err != nil {
		return err
	}
	message.Images = images
	return nil
}

// Message loads one stored message so image uploads can publish its complete
// chat event after the files have been attached.
func (s *Service) Message(id int64) (*model.Message, error) {
	return s.repo.GetMessage(id)
}
