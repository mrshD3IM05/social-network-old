package repository

import (
	"time"

	"sn-backend/internal/model"
)

const eventColumns = `
	e.id, e.group_id, e.creator_id, e.title, e.description, e.date_time, e.created_at,
	u.first_name, u.last_name`

func (r *EventRepository) CreateEvent(event *model.GroupEvent) error {
	result, err := r.db.Exec(
		`INSERT INTO group_events (group_id, creator_id, title, description, date_time)
		 VALUES (?, ?, ?, ?, ?)`,
		event.GroupID, event.CreatorID, event.Title, event.Description, event.DateTime,
	)
	if err != nil {
		return err
	}
	event.ID, err = result.LastInsertId()
	return err
}

// ListGroupEvents returns one page of a group's events (soonest first, after
// the event lastID) with going / not-going counts and the viewer's own choice.
func (r *EventRepository) ListGroupEvents(groupID, viewerID, lastID int64) ([]*model.EventListItem, error) {
	rows, err := r.db.Query(
		`SELECT `+eventColumns+`,
			COALESCE(SUM(CASE WHEN er.choice = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN er.choice = ? THEN 1 ELSE 0 END), 0),
			COALESCE(MAX(CASE WHEN er.user_id = ? THEN er.choice END), '')
		 FROM group_events e
		 JOIN users u ON u.id = e.creator_id
		 LEFT JOIN event_responses er ON er.event_id = e.id
		 WHERE e.group_id = ?
		   AND (? = 0 OR (e.date_time, e.id) > (SELECT date_time, id FROM group_events WHERE id = ?))
		 GROUP BY e.id
		 ORDER BY e.date_time, e.id
		 LIMIT ?`,
		model.EventChoiceGoing, model.EventChoiceNotGoing, viewerID, groupID,
		lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]*model.EventListItem, 0)
	for rows.Next() {
		// one Scan per row: event columns first, then the three aggregates
		item := new(model.EventListItem)
		if err := rows.Scan(
			&item.ID,
			&item.GroupID,
			&item.CreatorID,
			&item.Title,
			&item.Description,
			&item.DateTime,
			&item.CreatedAt,
			&item.CreatorFirstName,
			&item.CreatorLastName,
			&item.GoingCount,
			&item.NotGoingCount,
			&item.MyChoice,
		); err != nil {
			return nil, err
		}
		events = append(events, item)
	}
	return events, rows.Err()
}

// ListUpcomingEvents returns the next `limit` events, soonest first, across
// every group the viewer is a member of. The future check is done in Go so it
// does not depend on how the driver formats stored timestamps.
func (r *EventRepository) ListUpcomingEvents(viewerID int64, after time.Time, limit int) ([]*model.UpcomingEvent, error) {
	rows, err := r.db.Query(
		`SELECT e.id, e.group_id, e.creator_id, e.title, e.description, e.date_time, e.created_at, g.title
		 FROM group_events e
		 JOIN group_members gm ON gm.group_id = e.group_id AND gm.user_id = ?
		 JOIN groups g ON g.id = e.group_id
		 ORDER BY e.date_time, e.id`,
		viewerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]*model.UpcomingEvent, 0, limit)
	for rows.Next() && len(events) < limit {
		item := new(model.UpcomingEvent)
		if err := rows.Scan(
			&item.ID,
			&item.GroupID,
			&item.CreatorID,
			&item.Title,
			&item.Description,
			&item.DateTime,
			&item.CreatedAt,
			&item.GroupTitle,
		); err != nil {
			return nil, err
		}
		if item.DateTime.After(after) {
			events = append(events, item)
		}
	}
	return events, rows.Err()
}

// CountGroupEvents is how many events a group has, for the tab on its page.
func (r *EventRepository) CountGroupEvents(groupID int64) (int, error) {
	var count int
	err := r.QueryRow(`SELECT COUNT(*) FROM group_events WHERE group_id = ?`, groupID).Scan(&count)
	return count, err
}

func (r *EventRepository) GetGroupIDForEvent(eventID int64) (int64, error) {
	var groupID int64
	if err := r.QueryRow(`SELECT group_id FROM group_events WHERE id = ?`, eventID).Scan(&groupID); err != nil {
		return 0, notFound(err)
	}
	return groupID, nil
}

// SetEventResponse inserts or replaces the user's response to an event. The
// UNIQUE(event_id, user_id) constraint guarantees one row per user + event.
func (r *EventRepository) SetEventResponse(eventID, userID int64, choice string) error {
	result, err := r.db.Exec(
		`INSERT INTO event_responses (event_id, user_id, choice) VALUES (?, ?, ?)
		 ON CONFLICT (event_id, user_id) DO UPDATE SET choice = excluded.choice`,
		eventID, userID, choice,
	)
	if err != nil {
		return err
	}
	// Distinguish "no row changed" from success; the INSERT arm always
	// changes a row, so RowsAffected is 0 only on driver anomalies.
	if count, err := result.RowsAffected(); err == nil && count == 0 {
		return ErrExists
	}
	return nil
}

// DeleteEventResponse removes the user's answer to an event, if any.
func (r *EventRepository) DeleteEventResponse(eventID, userID int64) error {
	_, err := r.db.Exec(`DELETE FROM event_responses WHERE event_id = ? AND user_id = ?`, eventID, userID)
	return err
}

// EventResponseCounts returns going / not-going counts for one event.
func (r *EventRepository) EventResponseCounts(eventID int64) (going, notGoing int, err error) {
	err = r.QueryRow(
		`SELECT
			COALESCE(SUM(CASE WHEN choice = ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN choice = ? THEN 1 ELSE 0 END), 0)
		 FROM event_responses WHERE event_id = ?`,
		model.EventChoiceGoing, model.EventChoiceNotGoing, eventID,
	).Scan(&going, &notGoing)
	return going, notGoing, err
}
