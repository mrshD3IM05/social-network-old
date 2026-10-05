package filesvc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sn-backend/internal/model"
	"sn-backend/internal/repository"
)

const (
	MaxImageSize int64 = 10 << 20
	MaxImages          = 3
	// MaxImageSide is the largest width or height accepted, in pixels. It keeps
	// "image bombs" (a tiny file that expands to a huge picture) out.
	MaxImageSide = 8000
	// MaxRequestSize is the biggest upload body: every image plus the form fields.
	MaxRequestSize = MaxImageSize*MaxImages + 1<<20
	// MaxMemory is how much of an upload is held in memory while it is read.
	// Anything past it goes to a temporary file, so a few big uploads at the
	// same time cannot fill the server's memory.
	MaxMemory = 1 << 20
)

var (
	ErrInvalidImage    = errors.New("file: only JPEG, PNG, and GIF images are allowed")
	ErrFileTooLarge    = errors.New("file: image exceeds the 10 MB limit")
	ErrTooManyImages   = errors.New("file: a maximum of 3 images is allowed")
	ErrImageDimensions = errors.New("file: image is larger than 8000x8000 pixels")
)

// IsBadImage says whether err is the client's fault (answer 400).
func IsBadImage(err error) bool {
	return errors.Is(err, ErrInvalidImage) || errors.Is(err, ErrFileTooLarge) ||
		errors.Is(err, ErrTooManyImages) || errors.Is(err, ErrImageDimensions)
}

type Service struct {
	repo        *repository.FileRepository
	posts       *repository.PostRepository
	comments    *repository.CommentRepository
	messages    *repository.MessageRepository
	users       *repository.UserRepository
	storagePath string
}

func New(repo *repository.FileRepository, posts *repository.PostRepository, comments *repository.CommentRepository, messages *repository.MessageRepository, users *repository.UserRepository, storagePath string) *Service {
	return &Service{repo: repo, posts: posts, comments: comments, messages: messages, users: users, storagePath: storagePath}
}

// Upload stores one image (avatars, group pictures).
func (s *Service) Upload(ownerID int64, header *multipart.FileHeader, postID, messageID, commentID *int64) (*model.File, error) {
	files, err := s.UploadMany(ownerID, []*multipart.FileHeader{header}, postID, messageID, commentID)
	if err != nil {
		return nil, err
	}

	return files[0], nil
}

// UploadMany checks every image before it writes any, so a bad file in the
// batch never leaves the good ones half attached.
func (s *Service) UploadMany(ownerID int64, headers []*multipart.FileHeader, postID, messageID, commentID *int64) ([]*model.File, error) {
	if len(headers) == 0 {
		return nil, errors.New("file: at least one image is required")
	}
	if len(headers) > MaxImages {
		return nil, ErrTooManyImages
	}
	if err := s.checkTarget(ownerID, postID, messageID, commentID); err != nil {
		return nil, err
	}
	// the limit is per post, comment or message, not per request
	if postID != nil || messageID != nil || commentID != nil {
		count, err := s.repo.CountAttachedFiles(postID, messageID, commentID)
		if err != nil {
			return nil, err
		}
		if count+len(headers) > MaxImages {
			return nil, ErrTooManyImages
		}
	}

	types := make([]string, len(headers))
	for i, header := range headers {
		contentType, err := CheckImage(header)
		if err != nil {
			return nil, err
		}
		types[i] = contentType
	}

	files := make([]*model.File, 0, len(headers))
	for i, header := range headers {
		file, err := s.store(ownerID, header, types[i], postID, messageID, commentID)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// checkTarget: images can only be attached to your own post, comment or message.
func (s *Service) checkTarget(ownerID int64, postID, messageID, commentID *int64) error {
	if messageID != nil {
		allowed, err := s.messages.CanAttachToMessage(*messageID, ownerID)
		if err != nil {
			return err
		}
		if !allowed {
			return repository.ErrNotFound
		}
	}
	if postID != nil {
		post, err := s.posts.GetPost(*postID)
		if err != nil {
			return err
		}
		if post.AuthorID != ownerID {
			return repository.ErrNotFound
		}
	}
	if commentID != nil {
		comment, err := s.comments.GetComment(*commentID)
		if err != nil {
			return err
		}
		if comment.AuthorID != ownerID {
			return repository.ErrNotFound
		}
	}
	return nil
}

// CheckImages runs CheckImage on a whole batch.
func CheckImages(headers []*multipart.FileHeader) error {
	if len(headers) > MaxImages {
		return ErrTooManyImages
	}
	for _, header := range headers {
		if _, err := CheckImage(header); err != nil {
			return err
		}
	}
	return nil
}

// CheckImage makes sure the upload really is a JPEG, PNG or GIF picture and
// answers with its type. The file name and the browser's type are never
// trusted: the first bytes decide the type, then the image header is decoded
// to prove it matches and to read the size in pixels.
func CheckImage(header *multipart.FileHeader) (string, error) {
	if header == nil || header.Size <= 0 {
		return "", ErrInvalidImage
	}
	if header.Size > MaxImageSize {
		return "", ErrFileTooLarge
	}
	source, err := header.Open()
	if err != nil {
		return "", err
	}
	defer source.Close()

	contentType, err := detectImageType(source)
	if err != nil {
		return "", err
	}
	if !allowedImageType(contentType) {
		return "", ErrInvalidImage
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	config, format, err := image.DecodeConfig(source)
	if err != nil || "image/"+format != contentType {
		return "", ErrInvalidImage
	}
	if config.Width < 1 || config.Height < 1 {
		return "", ErrInvalidImage
	}
	if config.Width > MaxImageSide || config.Height > MaxImageSide {
		return "", ErrImageDimensions
	}
	return contentType, nil
}

// store writes one checked image to disk and records it.
func (s *Service) store(ownerID int64, header *multipart.FileHeader, contentType string, postID, messageID, commentID *int64) (*model.File, error) {
	source, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.storagePath, 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(s.storagePath, id)
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(destination, io.LimitReader(source, MaxImageSize+1)); err != nil {
		destination.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	file := &model.File{ID: id, StoragePath: path, OriginalName: filepath.Base(header.Filename), MIMEType: contentType, Size: header.Size, OwnerUserID: &ownerID, PostID: postID, CommentID: commentID, MessageID: messageID}
	if err := s.repo.CreateFile(file); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return file, nil
}

func (s *Service) Get(id string) (*model.File, error) { return s.repo.GetFile(id) }

func (s *Service) CanView(viewerID int64, id string) (bool, error) {
	return s.repo.CanViewFile(viewerID, id)
}

func (s *Service) SetAvatar(ownerID int64, header *multipart.FileHeader) (*model.User, error) {
	file, err := s.Upload(ownerID, header, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	user, err := s.users.GetUserByID(ownerID)
	if err != nil {
		return nil, err
	}
	user.Avatar = file.ID
	if err := s.users.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func detectImageType(source multipart.File) (string, error) {
	buffer := make([]byte, 512)
	count, err := source.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(buffer[:count]), nil
}

func allowedImageType(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/gif"
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
