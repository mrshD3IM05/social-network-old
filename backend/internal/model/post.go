package model

import "time"

const (
	PostPublic        = "public"
	PostFollowersOnly = "almost_private"
	PostSelected      = "private"
)

type Post struct {
	ID              int64     `json:"id"`
	AuthorID        int64     `json:"author_id"`
	AuthorFirstName string    `json:"author_first_name"`
	AuthorLastName  string    `json:"author_last_name"`
	AuthorNickname  string    `json:"author_nickname"`
	AuthorAvatar    string    `json:"author_avatar"`
	Content         string    `json:"content"`
	Privacy         string    `json:"privacy"`
	GroupID         *int64    `json:"group_id,omitempty"`
	GroupName       string    `json:"group_name,omitempty"`
	Images          []string  `json:"images"`
	Viewers         []int64   `json:"-"`
	Likes           int       `json:"likes"`
	Dislikes        int       `json:"dislikes"`
	MyReaction      string    `json:"my_reaction"`
	CommentCount    int       `json:"comment_count"`
	CreatedAt       time.Time `json:"created_at"`
}
