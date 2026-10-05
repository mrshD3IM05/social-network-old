package commentsvc

import (
	"errors"
	"strings"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/notificationsvc"
)

const maxContentLen = 2000

var (
	ErrInvalidContent = errors.New("comment: content is required (max 2000 chars)")
	ErrNotFound       = errors.New("comment: not found")
	ErrNoAccess       = errors.New("comment: no access to this post")
)

// Service uses the hub to notify a post's author of new comments.
type Service struct {
	repo          *repository.CommentRepository
	posts         *repository.PostRepository
	notifications notificationsvc.Notifier
}

func New(repo *repository.CommentRepository, posts *repository.PostRepository, notifications notificationsvc.Notifier) *Service {
	return &Service{repo: repo, posts: posts, notifications: notifications}
}

// List returns the comments of a post the viewer can see.
func (s *Service) List(viewerID, postID, lastID int64) ([]*model.Comment, error) {
	visible, err := s.posts.CanViewPost(viewerID, postID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrNoAccess
	}
	return s.repo.ListPostComments(postID, lastID)
}

// Create adds a comment to a post the viewer can see. Authorization goes
// through CanViewPost: post privacy for normal posts, group membership for
// group posts — a non-member cannot comment on a group post.
func (s *Service) Create(authorID, postID int64, content string) (*model.Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > maxContentLen {
		return nil, ErrInvalidContent
	}
	visible, err := s.posts.CanViewPost(authorID, postID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrNoAccess
	}

	comment := &model.Comment{PostID: postID, AuthorID: authorID, Content: content}
	if err := s.repo.CreateComment(comment); err != nil {
		return nil, err
	}

	created, err := s.reload(comment.ID, postID)
	if err != nil {
		return nil, err
	}

	// tell the post author (not when they comment on their own post)
	if post, err := s.posts.GetPost(postID); err == nil && post.AuthorID != authorID {
		s.notifications.Notify(&model.Notification{
			UserID:  post.AuthorID,
			Type:    model.NotificationCommentPost,
			ActorID: authorID,
			Content: created.AuthorFirstName + " " + created.AuthorLastName + " commented on your post",
		})
	}
	return created, nil
}

// reload re-reads the freshly inserted comment with its author fields and
// images, mirroring how the list endpoint renders comments.
func (s *Service) reload(commentID, postID int64) (*model.Comment, error) {
	comment, err := s.repo.GetComment(commentID)
	if err != nil {
		return nil, err
	}
	if comment.PostID != postID {
		return nil, repository.ErrNotFound
	}
	comment.Images, err = s.repo.ListCommentFileIDs(comment.ID)
	return comment, err
}

// Update changes the text of a comment the caller wrote.
func (s *Service) Update(authorID, commentID int64, content string) (*model.Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > maxContentLen {
		return nil, ErrInvalidContent
	}
	if err := s.repo.UpdateCommentOwned(commentID, authorID, content); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.repo.GetComment(commentID)
}

// Delete removes a comment. Its author may delete it, and so may the author of
// the post it sits under.
func (s *Service) Delete(userID, commentID int64) error {
	if err := s.repo.DeleteCommentOwned(commentID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
