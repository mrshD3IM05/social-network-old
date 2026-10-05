package repository

import "sn-backend/internal/model"

func (r *MessageRepository) CreateMessage(message *model.Message) error {
	result, err := r.db.Exec(`
		INSERT INTO messages (from_user_id, to_user_id, group_id, content)
		VALUES (?, ?, ?, ?)`, message.FromUserID, message.ToUserID, message.GroupID, message.Content)
	if err != nil {
		return err
	}
	message.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	return r.QueryRow(
		`SELECT m.created_at, u.first_name, u.last_name, COALESCE(u.avatar, '')
		 FROM messages m JOIN users u ON u.id = m.from_user_id WHERE m.id = ?`, message.ID,
	).Scan(&message.CreatedAt, &message.FromFirstName, &message.FromLastName, &message.FromAvatar)
}

// GetMessage returns one chat message for publishing after its HTTP images are
// attached to a message created through the WebSocket.
func (r *MessageRepository) GetMessage(id int64) (*model.Message, error) {
	message := new(model.Message)
	err := r.QueryRow(`
		SELECT m.id, m.from_user_id, m.to_user_id, m.group_id, m.content, m.created_at,
			u.first_name, u.last_name, COALESCE(u.avatar, '')
		FROM messages m JOIN users u ON u.id = m.from_user_id WHERE m.id = ?`, id,
	).Scan(&message.ID, &message.FromUserID, &message.ToUserID, &message.GroupID, &message.Content, &message.CreatedAt,
		&message.FromFirstName, &message.FromLastName, &message.FromAvatar)
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (r *MessageRepository) CanMessage(fromUserID int64, toUserID, groupID *int64) (bool, error) {
	if toUserID != nil {
		var allowed int
		// At least one of the two must follow the other.
		// A public profile is not enough on its own.
		err := r.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM users target
			WHERE target.id = ? AND EXISTS (
				SELECT 1 FROM follow_requests f
				WHERE (f.from_user_id = ? AND f.to_user_id = target.id OR f.from_user_id = target.id AND f.to_user_id = ?)
				AND f.status = 'accepted'
			)
		)`, *toUserID, fromUserID, fromUserID).Scan(&allowed)
		return allowed == 1, err
	}
	if groupID != nil {
		var allowed int
		err := r.QueryRow(`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?)`, *groupID, fromUserID).Scan(&allowed)
		return allowed == 1, err
	}
	return false, nil
}

// CanAttachToMessage: only the sender may add images to their own message.
func (r *MessageRepository) CanAttachToMessage(messageID, userID int64) (bool, error) {
	var allowed int
	err := r.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM messages m WHERE m.id = ? AND m.from_user_id = ?
	)`, messageID, userID).Scan(&allowed)
	return allowed == 1, err
}

// ListGroupMessages returns one older-to-newer page of a group chat.
func (r *MessageRepository) ListGroupMessages(groupID, lastID int64) ([]*model.Message, error) {
	rows, err := r.db.Query(`
		SELECT m.id, m.from_user_id, m.group_id, m.content, m.created_at,
			u.first_name, u.last_name, COALESCE(u.avatar, '')
		FROM (
			SELECT id, from_user_id, group_id, content, created_at
			FROM messages WHERE group_id = ? AND (? = 0 OR id < ?)
			ORDER BY id DESC LIMIT ?
		) m JOIN users u ON u.id = m.from_user_id
		ORDER BY m.id`, groupID, lastID, lastID, MessagePageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]*model.Message, 0)
	for rows.Next() {
		message := new(model.Message)
		if err := rows.Scan(&message.ID, &message.FromUserID, &message.GroupID, &message.Content, &message.CreatedAt,
			&message.FromFirstName, &message.FromLastName, &message.FromAvatar); err != nil {
			return nil, err
		}
		// a group message can carry pictures, like a private one
		message.Images, err = r.ListMessageFileIDs(message.ID)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
