package repository

import (
	"errors"

	"sn-backend/internal/model"
)

func (r *SessionRepository) CreateSession(session *model.Session) error {
	if session == nil {
		return errors.New("session is nil")
	}

	_, err := r.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		session.ID,
		session.UserID,
		session.ExpiresAt,
	)
	return err
}

func (r *SessionRepository) GetSession(id string) (*model.Session, error) {
	session := new(model.Session)
	err := r.QueryRow(
		`SELECT id, user_id, expires_at, created_at FROM sessions WHERE id = ?`,
		id,
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return session, nil
}

func (r *SessionRepository) DeleteSession(id string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}
