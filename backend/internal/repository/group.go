package repository

import (
	"database/sql"
	"errors"

	"sn-backend/internal/model"
)

const groupColumns = `g.id, g.creator_id, g.title, g.description, g.avatar, g.created_at`

func scanGroup(s scanner) (*model.Group, error) {
	group := new(model.Group)
	if err := s.Scan(
		&group.ID,
		&group.CreatorID,
		&group.Title,
		&group.Description,
		&group.Avatar,
		&group.CreatedAt,
	); err != nil {
		return nil, err
	}
	return group, nil
}

func (r *GroupRepository) CreateGroup(group *model.Group) error {
	if group == nil {
		return errors.New("group is nil")
	}

	result, err := r.db.Exec(
		`INSERT INTO groups (creator_id, title, description, avatar) VALUES (?, ?, ?, ?)`,
		group.CreatorID,
		group.Title,
		group.Description,
		group.Avatar,
	)
	if err != nil {
		return err
	}

	group.ID, err = result.LastInsertId()
	return err
}

func (r *GroupRepository) GetGroup(id int64) (*model.Group, error) {
	group, err := scanGroup(r.QueryRow(
		`SELECT `+groupColumns+` FROM groups g WHERE g.id = ?`,
		id,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return group, nil
}

func (r *GroupRepository) UpdateGroup(group *model.Group) error {
	if group == nil {
		return errors.New("group is nil")
	}

	_, err := r.db.Exec(
		`UPDATE groups SET creator_id = ?, title = ?, description = ?, avatar = ? WHERE id = ?`,
		group.CreatorID,
		group.Title,
		group.Description,
		group.Avatar,
		group.ID,
	)
	return err
}

// DeleteGroup removes the group. The foreign keys cascade to members,
// invitations, join requests, posts (and their comments), events, messages
// and notifications.
func (r *GroupRepository) DeleteGroup(id int64) error {
	_, err := r.db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

// ---------------------------------------------------------------- members

func (r *GroupRepository) AddGroupMember(groupID, userID int64) error {
	_, err := r.db.Exec(`INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`, groupID, userID)
	return err
}

// RemoveGroupMember deletes the membership and the old invitation / join
// request rows, so the user can be invited or ask to join again later.
func (r *GroupRepository) RemoveGroupMember(groupID, userID int64) error {
	return r.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM group_invitations WHERE group_id = ? AND to_user_id = ?`, groupID, userID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM group_join_requests WHERE group_id = ? AND user_id = ?`, groupID, userID)
		return err
	})
}

func (r *GroupRepository) IsGroupMember(groupID, userID int64) (bool, error) {
	var exists int
	err := r.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?)`,
		groupID, userID,
	).Scan(&exists)
	return exists == 1, err
}

// IsGroupCreator reports whether userID created groupID — the project's
// group-admin role (the schema has no separate admin column).
func (r *GroupRepository) IsGroupCreator(groupID, userID int64) (bool, error) {
	var exists int
	err := r.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM groups WHERE id = ? AND creator_id = ?)`,
		groupID, userID,
	).Scan(&exists)
	return exists == 1, err
}

func (r *GroupRepository) GroupMemberIDs(groupID int64) ([]int64, error) {
	rows, err := r.db.Query(`SELECT user_id FROM group_members WHERE group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		ids = append(ids, userID)
	}
	return ids, rows.Err()
}

const groupMemberColumns = `gm.group_id, gm.user_id, u.first_name, u.last_name, u.nickname, u.avatar, gm.created_at`

func scanGroupMember(s scanner) (*model.GroupMember, error) {
	member := new(model.GroupMember)
	if err := s.Scan(
		&member.GroupID,
		&member.UserID,
		&member.FirstName,
		&member.LastName,
		&member.Nickname,
		&member.Avatar,
		&member.JoinedAt,
	); err != nil {
		return nil, err
	}
	return member, nil
}

// GetGroupMembers returns one page of a group's members in joining order,
// after the member whose user id is lastID (0 for the first page).
func (r *GroupRepository) GetGroupMembers(groupID, lastID int64) ([]*model.GroupMember, error) {
	rows, err := r.db.Query(
		`SELECT `+groupMemberColumns+`
		 FROM group_members gm
		 JOIN users u ON u.id = gm.user_id
		 WHERE gm.group_id = ?
		   AND (? = 0 OR (gm.created_at, gm.user_id) >
				(SELECT created_at, user_id FROM group_members WHERE group_id = ? AND user_id = ?))
		 ORDER BY gm.created_at, gm.user_id
		 LIMIT ?`,
		groupID, lastID, groupID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]*model.GroupMember, 0)
	for rows.Next() {
		member, err := scanGroupMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// ----------------------------------------------------------- invitations

// the invitation, its group title and the name and photo of who sent it
const groupInvitationColumns = `gi.id, gi.group_id, g.title, gi.from_user_id, fu.first_name, fu.last_name, fu.avatar, gi.to_user_id, gi.status, gi.created_at`

func scanGroupInvitation(s scanner) (*model.GroupInvitation, error) {
	invitation := new(model.GroupInvitation)
	if err := s.Scan(
		&invitation.ID,
		&invitation.GroupID,
		&invitation.GroupTitle,
		&invitation.FromUserID,
		&invitation.FromFirstName,
		&invitation.FromLastName,
		&invitation.FromAvatar,
		&invitation.ToUserID,
		&invitation.Status,
		&invitation.CreatedAt,
	); err != nil {
		return nil, err
	}
	return invitation, nil
}

func (r *GroupRepository) CreateGroupInvitation(invitation *model.GroupInvitation) (*model.GroupInvitation, error) {
	result, err := r.db.Exec(
		`INSERT INTO group_invitations (group_id, from_user_id, to_user_id, status) VALUES (?, ?, ?, ?)`,
		invitation.GroupID, invitation.FromUserID, invitation.ToUserID, invitation.Status,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetGroupInvitationByID(id)
}

func (r *GroupRepository) GetGroupInvitationByID(id int64) (*model.GroupInvitation, error) {
	invitation, err := scanGroupInvitation(r.QueryRow(
		`SELECT `+groupInvitationColumns+`
		 FROM group_invitations gi
		 JOIN groups g ON g.id = gi.group_id
		 JOIN users fu ON fu.id = gi.from_user_id
		 WHERE gi.id = ?`,
		id,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return invitation, nil
}

// PendingGroupInvitation returns the current invitation for (group, toUser)
// regardless of status, so the service can decide between reuse and reject.
func (r *GroupRepository) PendingGroupInvitation(groupID, toUserID int64) (*model.GroupInvitation, error) {
	invitation, err := scanGroupInvitation(r.QueryRow(
		`SELECT `+groupInvitationColumns+`
		 FROM group_invitations gi
		 JOIN groups g ON g.id = gi.group_id
		 JOIN users fu ON fu.id = gi.from_user_id
		 WHERE gi.group_id = ? AND gi.to_user_id = ?
		 ORDER BY gi.id DESC LIMIT 1`,
		groupID, toUserID,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return invitation, nil
}

func (r *GroupRepository) GetPendingInvitationsForUser(userID, lastID int64) ([]*model.GroupInvitation, error) {
	rows, err := r.db.Query(
		`SELECT `+groupInvitationColumns+`
		 FROM group_invitations gi
		 JOIN groups g ON g.id = gi.group_id
		 JOIN users fu ON fu.id = gi.from_user_id
		 WHERE gi.to_user_id = ? AND gi.status = ?
		   AND (? = 0 OR (gi.created_at, gi.id) < (SELECT created_at, id FROM group_invitations WHERE id = ?))
		 ORDER BY gi.created_at DESC, gi.id DESC
		 LIMIT ?`,
		userID, model.GroupInvitationPending, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := make([]*model.GroupInvitation, 0)
	for rows.Next() {
		invitation, err := scanGroupInvitation(rows)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (r *GroupRepository) GetPendingInvitationsForGroup(groupID int64) ([]*model.GroupInvitation, error) {
	rows, err := r.db.Query(
		`SELECT `+groupInvitationColumns+`
		 FROM group_invitations gi
		 JOIN groups g ON g.id = gi.group_id
		 JOIN users fu ON fu.id = gi.from_user_id
		 WHERE gi.group_id = ? AND gi.status = ?
		 ORDER BY gi.created_at DESC, gi.id DESC`,
		groupID, model.GroupInvitationPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invitations := make([]*model.GroupInvitation, 0)
	for rows.Next() {
		invitation, err := scanGroupInvitation(rows)
		if err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (r *GroupRepository) DeleteGroupInvitation(id int64) error {
	_, err := r.db.Exec(`DELETE FROM group_invitations WHERE id = ?`, id)
	return err
}

func (r *GroupRepository) UpdateGroupInvitationStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE group_invitations SET status = ? WHERE id = ?`, status, id)
	return err
}

// --------------------------------------------------------- join requests

const groupJoinRequestColumns = `gj.id, gj.group_id, g.title, gj.user_id, u.first_name, u.last_name, u.nickname, COALESCE(u.avatar, ''), gj.status, gj.created_at`

func scanGroupJoinRequest(s scanner) (*model.GroupJoinRequest, error) {
	request := new(model.GroupJoinRequest)
	if err := s.Scan(
		&request.ID,
		&request.GroupID,
		&request.GroupTitle,
		&request.UserID,
		&request.FirstName,
		&request.LastName,
		&request.Nickname,
		&request.Avatar,
		&request.Status,
		&request.CreatedAt,
	); err != nil {
		return nil, err
	}
	return request, nil
}

func (r *GroupRepository) CreateGroupJoinRequest(request *model.GroupJoinRequest) (*model.GroupJoinRequest, error) {
	result, err := r.db.Exec(
		`INSERT INTO group_join_requests (group_id, user_id, status) VALUES (?, ?, ?)`,
		request.GroupID, request.UserID, request.Status,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetGroupJoinRequestByID(id)
}

func (r *GroupRepository) GetGroupJoinRequestByID(id int64) (*model.GroupJoinRequest, error) {
	request, err := scanGroupJoinRequest(r.QueryRow(
		`SELECT `+groupJoinRequestColumns+`
		 FROM group_join_requests gj
		 JOIN groups g ON g.id = gj.group_id
		 JOIN users u ON u.id = gj.user_id
		 WHERE gj.id = ?`,
		id,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return request, nil
}

// PendingGroupJoinRequest returns the current request for (group, user)
// regardless of status, so the service can decide between reuse and reject.
func (r *GroupRepository) PendingGroupJoinRequest(groupID, userID int64) (*model.GroupJoinRequest, error) {
	request, err := scanGroupJoinRequest(r.QueryRow(
		`SELECT `+groupJoinRequestColumns+`
		 FROM group_join_requests gj
		 JOIN groups g ON g.id = gj.group_id
		 JOIN users u ON u.id = gj.user_id
		 WHERE gj.group_id = ? AND gj.user_id = ?
		 ORDER BY gj.id DESC LIMIT 1`,
		groupID, userID,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return request, nil
}

func (r *GroupRepository) GetPendingJoinRequestsForGroup(groupID, lastID int64) ([]*model.GroupJoinRequest, error) {
	rows, err := r.db.Query(
		`SELECT `+groupJoinRequestColumns+`
		 FROM group_join_requests gj
		 JOIN groups g ON g.id = gj.group_id
		 JOIN users u ON u.id = gj.user_id
		 WHERE gj.group_id = ? AND gj.status = ?
		   AND (? = 0 OR (gj.created_at, gj.id) < (SELECT created_at, id FROM group_join_requests WHERE id = ?))
		 ORDER BY gj.created_at DESC, gj.id DESC
		 LIMIT ?`,
		groupID, model.GroupJoinPending, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]*model.GroupJoinRequest, 0)
	for rows.Next() {
		request, err := scanGroupJoinRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

// GetPendingJoinRequestsForCreator returns the pending join requests of every
// group created by userID, so they can be answered from the notifications page.
func (r *GroupRepository) GetPendingJoinRequestsForCreator(userID, lastID int64) ([]*model.GroupJoinRequest, error) {
	rows, err := r.db.Query(
		`SELECT `+groupJoinRequestColumns+`
		 FROM group_join_requests gj
		 JOIN groups g ON g.id = gj.group_id
		 JOIN users u ON u.id = gj.user_id
		 WHERE g.creator_id = ? AND gj.status = ?
		   AND (? = 0 OR (gj.created_at, gj.id) < (SELECT created_at, id FROM group_join_requests WHERE id = ?))
		 ORDER BY gj.created_at DESC, gj.id DESC
		 LIMIT ?`,
		userID, model.GroupJoinPending, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]*model.GroupJoinRequest, 0)
	for rows.Next() {
		request, err := scanGroupJoinRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *GroupRepository) DeleteGroupJoinRequest(id int64) error {
	_, err := r.db.Exec(`DELETE FROM group_join_requests WHERE id = ?`, id)
	return err
}

func (r *GroupRepository) UpdateGroupJoinRequestStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE group_join_requests SET status = ? WHERE id = ?`, status, id)
	return err
}

// GetGroupCreator loads a user and maps it to the public creator shape so
// sensitive fields (password hash, email, date of birth) never reach clients.
func (r *GroupRepository) GetGroupCreator(id int64) (*model.GroupCreator, error) {
	user, err := r.users.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	return &model.GroupCreator{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Avatar:    user.Avatar,
		Nickname:  user.Nickname,
		AboutMe:   user.AboutMe,
		Private:   user.Private,
	}, nil
}

// GetGroupIDForEvent returns the group an event belongs to.
func (r *GroupRepository) CountGroupMembers(groupID int64) (int, error) {
	var count int
	err := r.QueryRow(`SELECT COUNT(*) FROM group_members WHERE group_id = ?`, groupID).Scan(&count)
	return count, err
}

// --------------------------------------------------- detail and browsing

func (r *GroupRepository) GroupDetailPayload(groupID, viewerID int64) (*model.GroupDetail, error) {
	group, err := r.GetGroup(groupID)
	if err != nil {
		return nil, err
	}

	// only the first page rides along (the avatar row on the group page); the
	// whole list is GET /groups/{id}/members, 10 at a time
	members, err := r.GetGroupMembers(groupID, 0)
	if err != nil {
		return nil, err
	}

	detail := &model.GroupDetail{
		Group:   *group,
		Members: make([]model.GroupMember, 0, len(members)),
	}
	for _, member := range members {
		detail.Members = append(detail.Members, *member)
	}
	if err := r.QueryRow(`SELECT COUNT(*) FROM group_members WHERE group_id = ?`, groupID).Scan(&detail.MemberCount); err != nil {
		return nil, err
	}

	detail.Creator, err = r.GetGroupCreator(group.CreatorID)
	if err != nil {
		return nil, err
	}

	detail.IsCreator = viewerID == group.CreatorID
	if !detail.IsCreator {
		detail.IsMember, err = r.IsGroupMember(groupID, viewerID)
		if err != nil {
			return nil, err
		}
		pending, err := r.PendingGroupJoinRequest(groupID, viewerID)
		if err == nil {
			detail.PendingJoin = pending.Status == model.GroupJoinPending
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	return detail, nil
}

// GroupFilter narrows GET /groups to the groups the viewer is in, the ones
// they are not in, or every group.
type GroupFilter int

const (
	AllGroups GroupFilter = iota
	JoinedGroups
	OtherGroups
)

// GroupListPayload returns one page of groups, newest first, starting after
// the group lastID, each with its member count and the viewer's relation to it.
// One query: the counts and flags are subqueries on the page's rows only.
func (r *GroupRepository) GroupListPayload(viewerID int64, filter GroupFilter, lastID int64) ([]*model.GroupListItem, error) {
	rows, err := r.db.Query(
		`SELECT `+groupColumns+`,
			(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id),
			EXISTS(SELECT 1 FROM group_members m WHERE m.group_id = g.id AND m.user_id = ?) AS joined,
			EXISTS(SELECT 1 FROM group_join_requests j WHERE j.group_id = g.id AND j.user_id = ? AND j.status = ?)
		 FROM groups g
		 WHERE (? = 0 OR joined = (? = 1))
		   AND (? = 0 OR (g.created_at, g.id) < (SELECT created_at, id FROM groups WHERE id = ?))
		 ORDER BY g.created_at DESC, g.id DESC
		 LIMIT ?`,
		viewerID, viewerID, model.GroupJoinPending,
		filter, filter,
		lastID, lastID,
		PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*model.GroupListItem, 0)
	for rows.Next() {
		item := new(model.GroupListItem)
		if err := rows.Scan(
			&item.ID,
			&item.CreatorID,
			&item.Title,
			&item.Description,
			&item.Avatar,
			&item.CreatedAt,
			&item.MemberCount,
			&item.IsMember,
			&item.PendingJoin,
		); err != nil {
			return nil, err
		}
		item.IsCreator = item.CreatorID == viewerID
		items = append(items, item)
	}
	return items, rows.Err()
}

// ----------------------------------------------------------- consistency

// AcceptGroupInvitationTx creates the membership and marks the invitation
// accepted atomically. It returns ErrNotFound when the invitation is missing,
// ErrExists when it is no longer pending or the user is already a member.
func (r *GroupRepository) AcceptGroupInvitationTx(invitationID, userID int64) error {
	return r.withTx(func(tx *sql.Tx) error {
		var toUserID int64
		var status string
		err := tx.QueryRow(
			`SELECT to_user_id, status FROM group_invitations WHERE id = ?`,
			invitationID,
		).Scan(&toUserID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if toUserID != userID {
			return ErrNotOwner
		}
		if status != model.GroupInvitationPending {
			return ErrExists
		}

		var exists int
		if err := tx.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = (SELECT group_id FROM group_invitations WHERE id = ?) AND user_id = ?)`,
			invitationID, userID,
		).Scan(&exists); err != nil {
			return err
		}
		if exists == 1 {
			return ErrExists
		}

		if _, err := tx.Exec(
			`INSERT INTO group_members (group_id, user_id)
			 SELECT group_id, to_user_id FROM group_invitations WHERE id = ?`,
			invitationID,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`UPDATE group_invitations SET status = ? WHERE id = ?`,
			model.GroupInvitationAccept, invitationID,
		); err != nil {
			return err
		}
		return nil
	})
}

// AcceptGroupJoinRequestTx creates the membership and marks the join request
// accepted atomically. It returns ErrNotFound when the request is missing,
// ErrExists when it is no longer pending or the user is already a member.
func (r *GroupRepository) AcceptGroupJoinRequestTx(requestID, groupID int64) error {
	return r.withTx(func(tx *sql.Tx) error {
		var reqGroupID int64
		var reqUserID int64
		var status string
		err := tx.QueryRow(
			`SELECT group_id, user_id, status FROM group_join_requests WHERE id = ?`,
			requestID,
		).Scan(&reqGroupID, &reqUserID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if reqGroupID != groupID {
			return ErrNotFound
		}
		if status != model.GroupJoinPending {
			return ErrExists
		}

		var exists int
		if err := tx.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?)`,
			reqGroupID, reqUserID,
		).Scan(&exists); err != nil {
			return err
		}
		if exists == 1 {
			return ErrExists
		}

		if _, err := tx.Exec(
			`INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`,
			reqGroupID, reqUserID,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`UPDATE group_join_requests SET status = ? WHERE id = ?`,
			model.GroupJoinAccept, requestID,
		); err != nil {
			return err
		}
		return nil
	})
}

// RefuseGroupInvitationTx marks the invitation declined only when it is still
// pending and belongs to userID. Returns ErrNotFound / ErrNotOwner / ErrExists.
func (r *GroupRepository) RefuseGroupInvitationTx(invitationID, userID int64) error {
	return r.withTx(func(tx *sql.Tx) error {
		var toUserID int64
		var status string
		err := tx.QueryRow(
			`SELECT to_user_id, status FROM group_invitations WHERE id = ?`,
			invitationID,
		).Scan(&toUserID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if toUserID != userID {
			return ErrNotOwner
		}
		if status != model.GroupInvitationPending {
			return ErrExists
		}
		if _, err := tx.Exec(
			`UPDATE group_invitations SET status = ? WHERE id = ?`,
			model.GroupInvitationDecline, invitationID,
		); err != nil {
			return err
		}
		return nil
	})
}

// RefuseGroupJoinRequestTx marks the join request declined only when it is
// still pending and belongs to groupID. Returns ErrNotFound / ErrExists.
func (r *GroupRepository) RefuseGroupJoinRequestTx(requestID, groupID int64) error {
	return r.withTx(func(tx *sql.Tx) error {
		var reqGroupID int64
		var status string
		err := tx.QueryRow(
			`SELECT group_id, status FROM group_join_requests WHERE id = ?`,
			requestID,
		).Scan(&reqGroupID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if reqGroupID != groupID {
			return ErrNotFound
		}
		if status != model.GroupJoinPending {
			return ErrExists
		}
		if _, err := tx.Exec(
			`UPDATE group_join_requests SET status = ? WHERE id = ?`,
			model.GroupJoinDecline, requestID,
		); err != nil {
			return err
		}
		return nil
	})
}

// withTx runs fn inside a transaction, rolling back on error.
func (r *GroupRepository) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
