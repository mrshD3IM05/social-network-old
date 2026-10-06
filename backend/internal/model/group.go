package model

import "time"

const (
	GroupInvitationPending = "pending"
	GroupInvitationAccept  = "accepted"
	GroupInvitationDecline = "declined"

	GroupJoinPending = "pending"
	GroupJoinAccept  = "accepted"
	GroupJoinDecline = "declined"

	NotificationGroupInvite     = "group_invitation"
	NotificationGroupJoinReq    = "group_join_request"
	NotificationGroupInviteResp = "group_invite_response"
	NotificationGroupJoinResp   = "group_join_response"
	NotificationGroupRemoved    = "group_removed"
)

type Group struct {
	ID          int64     `json:"id"`
	CreatorID   int64     `json:"creator_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Avatar      string    `json:"avatar"`
	CreatedAt   time.Time `json:"created_at"`
}

type GroupMember struct {
	GroupID   int64     `json:"group_id"`
	UserID    int64     `json:"user_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	JoinedAt  time.Time `json:"joined_at"`
}

// GroupCreator is the public subset of the creator exposed on group detail —
// mirroring common.PublicUser so password/email/DOB never leak.
type GroupCreator struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Avatar    string `json:"avatar"`
	Nickname  string `json:"nickname"`
	AboutMe   string `json:"about_me"`
	Private   bool   `json:"private"`
}

type GroupInvitation struct {
	ID            int64     `json:"id"`
	GroupID       int64     `json:"group_id"`
	GroupTitle    string    `json:"group_title"`
	FromUserID    int64     `json:"from_user_id"`
	FromFirstName string    `json:"from_first_name"`
	FromLastName  string    `json:"from_last_name"`
	FromAvatar    string    `json:"from_avatar"`
	ToUserID      int64     `json:"to_user_id"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type GroupJoinRequest struct {
	ID         int64     `json:"id"`
	GroupID    int64     `json:"group_id"`
	GroupTitle string    `json:"group_title"`
	UserID     int64     `json:"user_id"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Nickname   string    `json:"nickname"`
	Avatar     string    `json:"avatar"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// GroupDetail is the payload of GET /groups/{group_id}: the group plus its
// creator, members and the caller's own relationship to the group.
type GroupDetail struct {
	Group
	Creator       *GroupCreator `json:"creator,omitempty"`
	Members       []GroupMember `json:"members"`
	MemberCount   int           `json:"member_count"`
	EventCount    int           `json:"event_count"` // members only, 0 for outsiders
	IsMember      bool          `json:"is_member"`
	IsCreator     bool          `json:"is_creator"`
	PendingInvite bool          `json:"pending_invite"`
	// InvitationID is the pending invitation to answer from the group page,
	// set only together with PendingInvite
	InvitationID int64 `json:"invitation_id,omitempty"`
	PendingJoin  bool  `json:"pending_join"`
}

type GroupListItem struct {
	Group
	MemberCount int  `json:"member_count"`
	IsMember    bool `json:"is_member"`
	PendingJoin bool `json:"pending_join"`
	IsCreator   bool `json:"is_creator"`
}
