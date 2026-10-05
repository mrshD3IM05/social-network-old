package repository

import "sn-backend/internal/model"

func (r *FollowRepository) GetFollowRequest(fromUserID, toUserID int64) (*model.FollowRequest, error) {
	follow := new(model.FollowRequest)
	err := r.QueryRow(
		`SELECT id, from_user_id, to_user_id, status, created_at
		 FROM follow_requests WHERE from_user_id = ? AND to_user_id = ?`,
		fromUserID, toUserID,
	).Scan(&follow.ID, &follow.FromUserID, &follow.ToUserID, &follow.Status, &follow.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return follow, nil
}

func (r *FollowRepository) CreateFollowRequest(fromUserID, toUserID int64, status string) (*model.FollowRequest, error) {
	result, err := r.db.Exec(
		`INSERT INTO follow_requests (from_user_id, to_user_id, status) VALUES (?, ?, ?)`,
		fromUserID, toUserID, status,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetFollowRequestByID(id)
}

func (r *FollowRepository) GetFollowRequestByID(id int64) (*model.FollowRequest, error) {
	follow := new(model.FollowRequest)
	err := r.QueryRow(
		`SELECT id, from_user_id, to_user_id, status, created_at FROM follow_requests WHERE id = ?`, id,
	).Scan(&follow.ID, &follow.FromUserID, &follow.ToUserID, &follow.Status, &follow.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	return follow, nil
}

func (r *FollowRepository) UpdateFollowStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE follow_requests SET status = ? WHERE id = ?`, status, id)
	return err
}

func (r *FollowRepository) DeleteFollow(fromUserID, toUserID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM follow_requests WHERE from_user_id = ? AND to_user_id = ?`,
		fromUserID, toUserID,
	)
	return err
}

func (r *FollowRepository) IsFollowing(fromUserID, toUserID int64) (bool, error) {
	var exists int
	err := r.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM follow_requests WHERE from_user_id = ? AND to_user_id = ? AND status = ?)`,
		fromUserID, toUserID, model.FollowAccepted,
	).Scan(&exists)
	return exists == 1, err
}

// ListFollowers returns the users who follow userID, ListFollowing the users
// userID follows. Only accepted requests count — a pending one is not a follow.
// Both order by name like the people directory, so the lists read the same on
// every call, and start after the user lastID (0: the first page). The viewer
// is who is asking, which is not necessarily the subject: both rows carry the
// relation the viewer has with them.
func (r *FollowRepository) ListFollowers(viewerID, userID, lastID int64) ([]*model.User, error) {
	return r.listFollowUsers(viewerID,
		`SELECT `+userColumns+viewerStateColumns+`
		 FROM follow_requests f
		 JOIN `+userTable+` ON v.id = f.from_user_id
		 `+viewerStateJoins+`
		 WHERE f.to_user_id = ? AND f.status = ? AND `+afterUserCondition+`
		 ORDER BY v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id
		 LIMIT ?`,
		userID, model.FollowAccepted, lastID, lastID, PageSize,
	)
}

func (r *FollowRepository) ListFollowing(viewerID, userID, lastID int64) ([]*model.User, error) {
	return r.listFollowUsers(viewerID,
		`SELECT `+userColumns+viewerStateColumns+`
		 FROM follow_requests f
		 JOIN `+userTable+` ON v.id = f.to_user_id
		 `+viewerStateJoins+`
		 WHERE f.from_user_id = ? AND f.status = ? AND `+afterUserCondition+`
		 ORDER BY v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id
		 LIMIT ?`,
		userID, model.FollowAccepted, lastID, lastID, PageSize,
	)
}

// ListMessageableUsers returns the users userID can start a private chat with.
// The rule is the one CanMessage applies before every message: at least one of
// the two follows the other, with an accepted request — in either direction, so
// a pending follow counts for nothing. Yourself is left out, like in /users.
// Newest conversation first, then by name, 10 at a time after the contact
// lastID. The
// caller is the viewer, so the relation on each row is the one they have with
// the person.
func (r *FollowRepository) ListMessageableUsers(userID, lastID int64) ([]*model.User, error) {
	// last_message is the id of the newest private message between the viewer
	// and person p (0 when none): ids only grow, so it orders like the time and
	// can be compared exactly when resuming after the contact lastID.
	lastMessage := func(p string) string {
		return `COALESCE((
			SELECT MAX(m.id) FROM messages m
			WHERE (m.from_user_id = ? AND m.to_user_id = ` + p + `.id)
			   OR (m.from_user_id = ` + p + `.id AND m.to_user_id = ?)
		), 0)`
	}
	// The placeholders are bound in the order they appear in the statement.
	rows, err := r.db.Query(
		`WITH cursor AS (
			SELECT `+lastMessage("u")+` AS k, u.first_name AS f, u.last_name AS l, u.id AS i
			FROM users u WHERE u.id = ?
		 )
		 SELECT `+userColumns+viewerStateColumns+`
		 FROM (SELECT uv.*, `+lastMessage("uv")+` AS last_message FROM user_view uv) v
		 `+viewerStateJoins+`
		 WHERE v.id != ?
		   AND EXISTS (
				SELECT 1
				FROM follow_requests f
				WHERE (
					(f.from_user_id = ? AND f.to_user_id = v.id)
					OR
					(f.from_user_id = v.id AND f.to_user_id = ?)
				)
				AND f.status = ?
		   )
		   AND (? = 0
				OR v.last_message < (SELECT k FROM cursor)
				OR (v.last_message = (SELECT k FROM cursor)
					AND (v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id) > (SELECT f, l, i FROM cursor)))
		 ORDER BY v.last_message DESC,
		   v.first_name COLLATE NOCASE,
		   v.last_name COLLATE NOCASE,
		   v.id
		 LIMIT ?`,
		userID, userID, lastID, // cursor
		userID, userID, // last_message
		userID, userID, // viewerStateJoins
		userID,
		userID, userID, model.FollowAccepted,
		lastID,
		PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*model.User{}
	for rows.Next() {
		user, err := scanUserForViewer(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// ListSuggestedUsers returns up to limit users the viewer has no accepted
// follow with in either direction (the "People you may know" panel), so the
// client does not have to fetch every user and every contact to filter them.
// Pending requests stay in, so the panel still shows who you asked.
func (r *FollowRepository) ListSuggestedUsers(userID int64, limit int) ([]*model.User, error) {
	return r.listFollowUsers(userID,
		`SELECT `+userColumns+viewerStateColumns+`
		 FROM `+userTable+`
		 `+viewerStateJoins+`
		 WHERE v.id != ?
		   AND COALESCE(outgoing.status, '') != ?
		   AND COALESCE(incoming.status, '') != ?
		 ORDER BY v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id
		 LIMIT ?`,
		userID, model.FollowAccepted, model.FollowAccepted, limit,
	)
}

// listFollowUsers runs one of the people queries above. The query has to be
// written with viewerStateJoins and viewerStateColumns, and the viewer goes in
// ahead of every argument it carries, because those two placeholders are the
// first ones in the statement.
func (r *FollowRepository) listFollowUsers(viewerID int64, query string, args ...any) ([]*model.User, error) {
	rows, err := r.db.Query(query, append([]any{viewerID, viewerID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []*model.User{}
	for rows.Next() {
		user, err := scanUserForViewer(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// ListPendingFollowRequests returns the follow requests waiting for userID to
// accept or decline them, with the user who sent each one, carrying the
// relation userID has with that sender.
func (r *FollowRepository) ListPendingFollowRequests(viewerID, userID, lastID int64) ([]*model.FollowRequest, error) {
	rows, err := r.db.Query(
		`SELECT f.id, f.created_at, `+userColumns+viewerStateColumns+`
		 FROM follow_requests f
		 JOIN `+userTable+` ON v.id = f.from_user_id
		 `+viewerStateJoins+`
		 WHERE f.to_user_id = ? AND f.status = ? AND (? = 0 OR f.id < ?)
		 ORDER BY f.id DESC
		 LIMIT ?`,
		viewerID, viewerID, userID, model.FollowPending, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []*model.FollowRequest{}
	for rows.Next() {
		request := &model.FollowRequest{ToUserID: userID, Status: model.FollowPending}
		// the request id and time come first, then the sender as the caller
		// relates to them
		from, err := scanUserRow(rows, []any{&request.ID, &request.CreatedAt}, true)
		if err != nil {
			return nil, err
		}
		request.From = from
		request.FromUserID = from.ID
		requests = append(requests, request)
	}
	return requests, rows.Err()
}
