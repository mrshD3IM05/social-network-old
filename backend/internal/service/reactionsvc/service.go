package reactionsvc

import (
	"errors"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"
)

var (
	ErrInvalidReaction = errors.New("post: invalid reaction")
	ErrPostNotFound    = errors.New("post: not found")
)

type Service struct {
	repo  *repository.ReactionRepository
	posts *repository.PostRepository
}

func New(repo *repository.ReactionRepository, posts *repository.PostRepository) *Service {
	return &Service{repo: repo, posts: posts}
}

func (s *Service) React(userID, postID int64, reaction string) (*model.ReactionSummary, error) {
	if reaction != model.ReactionLike && reaction != model.ReactionDislike {
		return nil, ErrInvalidReaction
	}
	if err := s.checkVisible(userID, postID); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetReaction(model.ReactionTargetPost, postID, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil && existing.Reaction == reaction {
		if err := s.repo.DeleteReaction(model.ReactionTargetPost, postID, userID); err != nil {
			return nil, err
		}
	} else if err := s.repo.SetReaction(model.ReactionTargetPost, postID, userID, reaction); err != nil {
		return nil, err
	}
	return s.repo.GetReactionSummary(model.ReactionTargetPost, postID, userID)
}

func (s *Service) Unreact(userID, postID int64) (*model.ReactionSummary, error) {
	if err := s.checkVisible(userID, postID); err != nil {
		return nil, err
	}
	if err := s.repo.DeleteReaction(model.ReactionTargetPost, postID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetReactionSummary(model.ReactionTargetPost, postID, userID)
}

func (s *Service) checkVisible(viewerID, postID int64) error {
	visible, err := s.posts.CanViewPost(viewerID, postID)
	if err != nil {
		return err
	}
	if !visible {
		return ErrPostNotFound
	}
	return nil
}
