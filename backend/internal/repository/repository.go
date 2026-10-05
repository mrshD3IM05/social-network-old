package repository

import (
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrNotFound = errors.New("repository: not found")
	ErrExists   = errors.New("repository: already exists")
	ErrNotOwner = errors.New("repository: not the owner")
)

// PageSize is how many items a list endpoint answers at once ("10 by 10").
// The next ones are asked for with ?last=<id of the last item already shown>.
const PageSize = 10

// MessagePageSize is the chat history page: 10 like every other list, older
// ones asked for with ?last=<id of the oldest message shown>.
const MessagePageSize = PageSize

type dbStore struct {
	db *sql.DB
}

type Repositories struct {
	Users         *UserRepository
	Follows       *FollowRepository
	Posts         *PostRepository
	Reactions     *ReactionRepository
	Comments      *CommentRepository
	Events        *EventRepository
	Files         *FileRepository
	Groups        *GroupRepository
	Messages      *MessageRepository
	Notifications *NotificationRepository
	Sessions      *SessionRepository
}

type UserRepository struct{ *dbStore }
type FollowRepository struct{ *dbStore }
type PostRepository struct {
	*dbStore
	comments  *CommentRepository
	reactions *ReactionRepository
}
type ReactionRepository struct{ *dbStore }
type CommentRepository struct{ *dbStore }
type EventRepository struct{ *dbStore }
type FileRepository struct{ *dbStore }
type GroupRepository struct {
	*dbStore
	users *UserRepository
}
type MessageRepository struct{ *dbStore }
type NotificationRepository struct{ *dbStore }
type SessionRepository struct{ *dbStore }

func New(db *sql.DB) *Repositories {
	store := &dbStore{db: db}
	users := &UserRepository{dbStore: store}
	reactions := &ReactionRepository{dbStore: store}
	comments := &CommentRepository{dbStore: store}
	return &Repositories{
		Users:         users,
		Follows:       &FollowRepository{dbStore: store},
		Posts:         &PostRepository{dbStore: store, comments: comments, reactions: reactions},
		Reactions:     reactions,
		Comments:      comments,
		Events:        &EventRepository{dbStore: store},
		Files:         &FileRepository{dbStore: store},
		Groups:        &GroupRepository{dbStore: store, users: users},
		Messages:      &MessageRepository{dbStore: store},
		Notifications: &NotificationRepository{dbStore: store},
		Sessions:      &SessionRepository{dbStore: store},
	}
}

func (r *dbStore) QueryRow(query string, args ...any) *sql.Row {
	return r.db.QueryRow(query, args...)
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// placeholders returns "?, ?, ?" for n arguments (IN clauses).
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// int64sToAny converts an ID slice into query arguments.
func int64sToAny(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}
