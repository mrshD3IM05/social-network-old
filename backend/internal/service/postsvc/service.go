package postsvc

import (
	"errors"
	"sn-backend/internal/model"
	"sn-backend/internal/repository"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidPrivacy = errors.New("post: invalid privacy")
	ErrInvalidContent = errors.New("post: content is required (max 1000 chars)")
	ErrNotFound       = errors.New("post: not found")
	ErrNotGroupMember = errors.New("post: only group members can do that")
	ErrForbidden      = errors.New("post: only the author or the group creator can delete a group post")
	ErrInvalidViewers = errors.New("post: a private post needs at least one of your followers chosen")
	ErrInvalidFiles   = errors.New("post: attachments must already belong to this post")
)

type Service struct {
	repo    *repository.PostRepository
	follows *repository.FollowRepository
	groups  *repository.GroupRepository
}

func New(repo *repository.PostRepository, follows *repository.FollowRepository, groups *repository.GroupRepository) *Service {
	return &Service{repo: repo, follows: follows, groups: groups}
}

type ViewerService struct{ repo *repository.PostRepository }

func NewViewerService(repo *repository.PostRepository) *ViewerService {
	return &ViewerService{repo: repo}
}

const maxContentLen = 1000

func checkContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > maxContentLen {
		return "", ErrInvalidContent
	}
	return content, nil
}

func checkContentOrAttachments(content string, hasAttachments bool) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" && hasAttachments {
		return "", nil
	}
	return checkContent(content)
}

func validPrivacy(privacy string) bool {
	return privacy == model.PostPublic || privacy == model.PostFollowersOnly || privacy == model.PostSelected
}
func (s *Service) checkViewers(authorID int64, viewers []int64) error {
	if len(viewers) == 0 {
		return ErrInvalidViewers
	}
	for _, viewer := range viewers {
		following, err := s.follows.IsFollowing(viewer, authorID)
		if err != nil {
			return err
		}
		if !following {
			return ErrInvalidViewers
		}
	}
	return nil
}

func (s *Service) CreatePost(userID int64, groupID *int64, content, privacy string, viewers []int64, hasAttachments bool) (*model.Post, error) {
	var err error
	if groupID != nil {
		member, err := s.groups.IsGroupMember(*groupID, userID)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrNotGroupMember
		}
		privacy = model.PostPublic
		viewers = nil
		content, err = checkContentOrAttachments(content, hasAttachments)
		if err != nil {
			return nil, err
		}
	} else {
		content, err = checkContentOrAttachments(content, hasAttachments)
		if err != nil {
			return nil, err
		}
		if !validPrivacy(privacy) {
			return nil, ErrInvalidPrivacy
		}
		if privacy == model.PostSelected {
			if err := s.checkViewers(userID, viewers); err != nil {
				return nil, err
			}
		}
	}
	post := &model.Post{AuthorID: userID, Content: content, Privacy: privacy, GroupID: groupID}
	if err := s.repo.CreatePost(post); err != nil {
		return nil, err
	}
	if groupID == nil && privacy == model.PostSelected {
		if err := s.repo.SetPostViewers(post.ID, viewers); err != nil {
			return nil, err
		}
	}
	return s.loadPost(post.ID, userID)
}

func (s *Service) GetPost(viewerID, postID int64) (*model.Post, error) {
	visible, err := s.repo.CanViewPost(viewerID, postID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrNotFound
	}
	return s.loadPost(postID, viewerID)
}

func (s *Service) loadPost(postID, viewerID int64) (*model.Post, error) {
	post, err := s.repo.GetPost(postID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.repo.LoadPostReactions(post, viewerID); err != nil {
		return nil, err
	}
	return post, nil
}

func (s *Service) ListPosts(viewerID, authorID int64, groupID *int64, lastID int64) ([]*model.Post, error) {
	if groupID != nil {
		member, err := s.groups.IsGroupMember(*groupID, viewerID)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrNotGroupMember
		}
		return s.repo.ListGroupPosts(*groupID, viewerID, lastID)
	}
	return s.repo.ListVisiblePosts(viewerID, authorID, lastID)
}

func (s *Service) UpdatePost(userID, postID int64, groupID *int64, content, privacy string, viewers []int64, attachments []string) (*model.Post, error) {
	post, err := s.repo.GetPost(postID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if groupID == nil {
		if post.GroupID != nil || post.AuthorID != userID {
			return nil, ErrNotFound
		}
		removedFileIDs, err := filesToRemove(post.Images, attachments)
		if err != nil {
			return nil, err
		}
		content, err = checkContent(content)
		if err != nil {
			return nil, err
		}
		if !validPrivacy(privacy) {
			return nil, ErrInvalidPrivacy
		}
		if privacy == model.PostSelected {
			if len(viewers) == 0 {
				if post.Privacy != model.PostSelected {
					return nil, ErrInvalidViewers
				}
				viewers, err = s.repo.ListPostViewers(postID)
				if err != nil {
					return nil, err
				}
			} else if err := s.checkViewers(userID, viewers); err != nil {
				return nil, err
			}
		} else {
			viewers = nil
		}
		post.Content = content
		post.Privacy = privacy
		if err := s.repo.UpdatePostOwned(post, userID, removedFileIDs); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if err := s.repo.SetPostViewers(postID, viewers); err != nil {
			return nil, err
		}
		return s.loadPost(postID, userID)
	}

	if post.GroupID == nil || *post.GroupID != *groupID {
		return nil, ErrNotFound
	}
	member, err := s.groups.IsGroupMember(*groupID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotGroupMember
	}
	if post.AuthorID != userID {
		return nil, ErrForbidden
	}
	removedFileIDs, err := filesToRemove(post.Images, attachments)
	if err != nil {
		return nil, err
	}
	content, err = checkContent(content)
	if err != nil {
		return nil, err
	}
	post.Content = content
	post.Privacy = model.PostPublic
	if err := s.repo.UpdatePostOwned(post, userID, removedFileIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.loadPost(postID, userID)
}

func filesToRemove(current, keep []string) ([]string, error) {
	if keep == nil {
		return nil, nil
	}
	currentSet := make(map[string]struct{}, len(current))
	for _, fileID := range current {
		currentSet[fileID] = struct{}{}
	}
	keepSet := make(map[string]struct{}, len(keep))
	for _, fileID := range keep {
		if _, exists := currentSet[fileID]; !exists {
			return nil, ErrInvalidFiles
		}
		keepSet[fileID] = struct{}{}
	}
	removed := make([]string, 0, len(current)-len(keepSet))
	for _, fileID := range current {
		if _, exists := keepSet[fileID]; !exists {
			removed = append(removed, fileID)
		}
	}
	return removed, nil
}

func (s *Service) DeletePost(userID, postID int64, groupID *int64) error {
	post, err := s.repo.GetPost(postID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if groupID == nil {
		if post.GroupID != nil || post.AuthorID != userID {
			return ErrNotFound
		}
		if err := s.repo.DeletePostOwned(postID, userID); errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		} else {
			return err
		}
	}
	if post.GroupID == nil || *post.GroupID != *groupID {
		return ErrNotFound
	}
	creator, err := s.groups.IsGroupCreator(*groupID, userID)
	if err != nil {
		return err
	}
	member, err := s.groups.IsGroupMember(*groupID, userID)
	if err != nil {
		return err
	}
	if !creator && !member {
		return ErrNotGroupMember
	}
	if !creator && post.AuthorID != userID {
		return ErrForbidden
	}
	if err := s.repo.DeletePost(postID); errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	} else {
		return err
	}
}

func (s *ViewerService) Viewers(userID, postID int64) ([]int64, error) {
	post, err := s.repo.GetPost(postID)
	if err != nil || post.AuthorID != userID {
		return nil, ErrNotFound
	}
	if post.Privacy == model.PostFollowersOnly {
		return post.Viewers, nil
	}
	return s.repo.ListPostViewers(postID)
}
