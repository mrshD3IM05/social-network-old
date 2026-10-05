package repository

import (
	"errors"
	"strings"

	"sn-backend/internal/model"
)

// Every user read goes through user_view (migration 000020): a user plus the
// two follow counts, so no query has to count them again. The view is aliased
// v, which lets one column list fit a plain lookup and a join alike, in the
// order scanUser reads it.
const userTable = "user_view v"

const userColumns = "v.id, v.email, v.password, v.first_name, v.last_name, v.date_of_birth, v.avatar, v.nickname, v.about_me, v.private, v.created_at, v.followers, v.following, v.post_count"

// viewerStateColumns and viewerStateJoins are the half of the relation that
// user_view cannot hold: is_followed and is_following depend on whoever is
// asking, and a view is never handed an id. outgoing is the viewer following
// the row, incoming the row following the viewer. follow_requests is unique per
// pair, so neither join matches more than one row. Both join arguments are the
// viewer and come first, ahead of the arguments of the rest of the query.
//
// They carry the stored status rather than a number, so the one place mapping a
// status onto model.FollowState stays model.FollowState.
const viewerStateColumns = ", COALESCE(outgoing.status, ''), COALESCE(incoming.status, '')"

const viewerStateJoins = `
	LEFT JOIN follow_requests outgoing ON outgoing.from_user_id = ? AND outgoing.to_user_id = v.id
	LEFT JOIN follow_requests incoming ON incoming.from_user_id = v.id AND incoming.to_user_id = ?`

type scanner interface {
	Scan(dest ...any) error
}

// scanUserRow reads one row of a user query: the columns the query put in front
// of the user, if any, then the user_view columns and, when withViewer, the two
// relation statuses. They all have to reach the one Scan call a row is given —
// a rows value never hands the same row out twice.
func scanUserRow(s scanner, leading []any, withViewer bool) (*model.User, error) {
	user := new(model.User)
	var private int
	var outgoing, incoming string

	dest := make([]any, 0, len(leading)+16)
	dest = append(dest, leading...)
	dest = append(dest,
		&user.ID,
		&user.Email,
		&user.Password,
		&user.FirstName,
		&user.LastName,
		&user.DateOfBirth,
		&user.Avatar,
		&user.Nickname,
		&user.AboutMe,
		&private,
		&user.CreatedAt,
		&user.Followers,
		&user.Following,
		&user.PostCount,
	)
	if withViewer {
		dest = append(dest, &outgoing, &incoming)
	}

	if err := s.Scan(dest...); err != nil {
		return nil, err
	}

	user.Private = private == 1
	if withViewer {
		user.IsFollowed = model.FollowState(outgoing)
		user.IsFollowing = model.FollowState(incoming)
	}
	return user, nil
}

// scanUser reads a plain user_view row: the profile and the two follow counts
// the view already carries. It takes no viewer, so both relations stay
// FollowStateNone — the reads that need them go through scanUserForViewer.
func scanUser(s scanner) (*model.User, error) {
	return scanUserRow(s, nil, false)
}

// scanUserForViewer reads everything scanUser does plus the relations of the
// viewer with the row. Only a query joined with viewerStateJoins and selecting
// viewerStateColumns may be scanned by it.
func scanUserForViewer(s scanner) (*model.User, error) {
	return scanUserRow(s, nil, true)
}

func (r *UserRepository) CreateUser(user *model.User) error {
	if user == nil {
		return errors.New("user is nil")
	}

	result, err := r.db.Exec(
		`INSERT INTO users (email, password, first_name, last_name, date_of_birth, avatar, nickname, about_me, private) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.Email,
		user.Password,
		user.FirstName,
		user.LastName,
		user.DateOfBirth,
		user.Avatar,
		user.Nickname,
		user.AboutMe,
		boolToInt(user.Private),
	)
	if err != nil {
		return err
	}

	user.ID, err = result.LastInsertId()
	return err
}

// GetUserForViewer reads one user with the relations the viewer has with
// them. The two join arguments come first, ahead of the id.
func (r *UserRepository) GetUserForViewer(viewerID, id int64) (*model.User, error) {
	user, err := scanUserForViewer(r.QueryRow(
		`SELECT `+userColumns+viewerStateColumns+` FROM `+userTable+viewerStateJoins+` WHERE v.id = ?`,
		viewerID, viewerID, id,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return user, nil
}

func (r *UserRepository) GetUserByID(id int64) (*model.User, error) {
	user, err := scanUser(r.QueryRow(`SELECT `+userColumns+` FROM `+userTable+` WHERE v.id = ?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return user, nil
}

// afterUserCondition keeps the users that come after the user lastID in name
// order (first name, last name, id), so a page starts right after the one
// before. It takes lastID twice: 0 means the first page.
const afterUserCondition = `(? = 0 OR (v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id) >
	(SELECT first_name, last_name, id FROM users WHERE id = ?))`

// ListUsers returns one page of the users except the viewer whose name or
// nickname contains search (empty search keeps everyone), ordered by name,
// starting after the user lastID. Every row carries the relation the viewer
// has with it, so the caller needs no follow query of its own.
func (r *UserRepository) ListUsers(viewerID int64, search string, lastID int64) ([]*model.User, error) {
	// % and _ are wildcards in LIKE, so they are escaped to be searched as text
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search) + "%"
	rows, err := r.db.Query(
		`SELECT `+userColumns+viewerStateColumns+` FROM `+userTable+viewerStateJoins+`
		 WHERE v.id != ? AND (v.first_name || ' ' || v.last_name || ' ' || v.nickname) LIKE ? ESCAPE '\'
		 AND `+afterUserCondition+`
		 ORDER BY v.first_name COLLATE NOCASE, v.last_name COLLATE NOCASE, v.id
		 LIMIT ?`,
		viewerID, viewerID, viewerID, pattern, lastID, lastID, PageSize,
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

func (r *UserRepository) GetUserByEmail(email string) (*model.User, error) {
	user, err := scanUser(r.QueryRow(`SELECT `+userColumns+` FROM `+userTable+` WHERE v.email = ?`, email))
	if err != nil {
		return nil, notFound(err)
	}
	return user, nil
}

func (r *UserRepository) GetUserByNickname(nickname string) (*model.User, error) {
	user, err := scanUser(r.QueryRow(`SELECT `+userColumns+` FROM `+userTable+` WHERE v.nickname = ?`, nickname))
	if err != nil {
		return nil, notFound(err)
	}
	return user, nil
}

func (r *UserRepository) UpdateUser(user *model.User) error {
	if user == nil {
		return errors.New("user is nil")
	}

	_, err := r.db.Exec(
		`UPDATE users SET first_name = ?, last_name = ?, date_of_birth = ?, avatar = ?, nickname = ?, about_me = ?, private = ? WHERE id = ?`,
		user.FirstName,
		user.LastName,
		user.DateOfBirth,
		user.Avatar,
		user.Nickname,
		user.AboutMe,
		boolToInt(user.Private),
		user.ID,
	)
	return err
}

// SetUserPrivate flips only the privacy column, so turning a profile
// public or private cannot touch any other field.
func (r *UserRepository) SetUserPrivate(id int64, private bool) error {
	_, err := r.db.Exec(`UPDATE users SET private = ? WHERE id = ?`, boolToInt(private), id)
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
