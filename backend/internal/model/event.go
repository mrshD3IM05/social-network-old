package model

import "time"

const (
	EventChoiceGoing    = "going"
	EventChoiceNotGoing = "not_going"

	NotificationEventCreated = "event_created"
)

type GroupEvent struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	CreatorID   int64     `json:"creator_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DateTime    time.Time `json:"date_time"`
	CreatedAt   time.Time `json:"created_at"`
}

// UpcomingEvent is one row of GET /events/upcoming: an event from one of the
// viewer's groups, with the group title so the side panel needs no extra call.
type UpcomingEvent struct {
	GroupEvent
	GroupTitle string `json:"group_title"`
}

// EventListItem is one row of GET /groups/{group_id}/events: the event plus the
// response counts and the viewer's own choice ("", "going" or "not_going").
type EventListItem struct {
	GroupEvent
	CreatorFirstName string `json:"creator_first_name"`
	CreatorLastName  string `json:"creator_last_name"`
	GoingCount       int    `json:"going_count"`
	NotGoingCount    int    `json:"not_going_count"`
	MyChoice         string `json:"my_choice"`
}
