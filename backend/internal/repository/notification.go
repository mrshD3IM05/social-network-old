package repository

import "sn-backend/internal/model"

// the notification columns plus the name and photo of the user who caused it
const notificationSelect = `SELECT n.id, n.user_id, n.type, n.actor_id, u.first_name, u.last_name, u.avatar,
	n.content, n.group_id, n.read, n.created_at
	FROM notifications n JOIN users u ON u.id = n.actor_id`

func scanNotification(s scanner) (*model.Notification, error) {
	n := new(model.Notification)
	err := s.Scan(&n.ID, &n.UserID, &n.Type, &n.ActorID, &n.ActorFirstName, &n.ActorLastName, &n.ActorAvatar,
		&n.Content, &n.GroupID, &n.Read, &n.CreatedAt)
	return n, err
}

// CreateNotification stores the notification and fills in the rest of its
// fields (id, actor name, date) so it can be sent to the user as is.
func (r *NotificationRepository) CreateNotification(n *model.Notification) error {
	result, err := r.db.Exec(
		`INSERT INTO notifications (user_id, type, actor_id, content, group_id) VALUES (?, ?, ?, ?, ?)`,
		n.UserID, n.Type, n.ActorID, n.Content, n.GroupID,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	stored, err := scanNotification(r.QueryRow(notificationSelect+` WHERE n.id = ?`, id))
	if err != nil {
		return err
	}
	*n = *stored
	return nil
}

// DeleteJoinRequestNotification drops the "X requested to join" notification the
// requester sent to the group creator, so a cancelled request stops showing in
// the creator's list. The notification belongs to the creator (user_id) but is
// caused by the requester, so the requester is matched on actor_id — filtering
// on user_id would match nothing, since the requester is not the recipient.
func (r *NotificationRepository) DeleteJoinRequestNotification(userID, groupID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM notifications WHERE actor_id = ? AND group_id = ? AND type = ?`,
		userID, groupID, model.NotificationGroupJoinReq,
	)
	return err
}

// ListNotifications returns one page of a user's notifications, newest first,
// starting after the notification lastID (0: the first page).
func (r *NotificationRepository) ListNotifications(userID, lastID int64) ([]*model.Notification, error) {
	rows, err := r.db.Query(notificationSelect+` WHERE n.user_id = ? AND (? = 0 OR n.id < ?) ORDER BY n.id DESC LIMIT ?`, userID, lastID, lastID, PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []*model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func (r *NotificationRepository) CountUnreadNotifications(userID int64) (int, error) {
	var count int
	err := r.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0`, userID).Scan(&count)
	return count, err
}

func (r *NotificationRepository) MarkNotificationsRead(userID int64) error {
	_, err := r.db.Exec(`UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0`, userID)
	return err
}
