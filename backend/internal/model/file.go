package model

import "time"

type File struct {
	ID           string    `json:"id"`
	StoragePath  string    `json:"-"`
	OriginalName string    `json:"original_name"`
	MIMEType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	OwnerUserID  *int64    `json:"owner_user_id"`
	PostID       *int64    `json:"post_id,omitempty"`
	CommentID    *int64    `json:"comment_id,omitempty"`
	MessageID    *int64    `json:"message_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
