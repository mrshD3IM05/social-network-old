package repository

import (
	"encoding/json"

	"sn-backend/internal/model"
)

func (r *PostRepository) CreatePost(post *model.Post) error {
	result, err := r.db.Exec(
		`INSERT INTO posts (author_id, content, privacy, group_id) VALUES (?, ?, ?, ?)`,
		post.AuthorID, post.Content, post.Privacy, post.GroupID,
	)
	if err != nil {
		return err
	}
	post.ID, err = result.LastInsertId()
	return err
}

// SetPostViewers replaces the users allowed to see a "private" post.
func (r *PostRepository) SetPostViewers(postID int64, userIDs []int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM post_visibility WHERE post_id = ?`, postID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO post_visibility (post_id, user_id) VALUES (?, ?)`, postID, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPostViewers returns the ids of the users a "private" post was shared
// with who still follow its author, so its author can see them already ticked
// when editing.
func (r *PostRepository) ListPostViewers(postID int64) ([]int64, error) {
	rows, err := r.db.Query(`
		SELECT v.user_id
		FROM post_visibility v
		JOIN posts p ON p.id = v.post_id
		JOIN follow_requests f ON f.from_user_id = v.user_id
			AND f.to_user_id = p.author_id AND f.status = ?
		WHERE v.post_id = ?
		ORDER BY v.user_id`, model.FollowAccepted, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

const postViewColumns = `
	p.id, p.author_id, p.content, p.privacy, p.group_id, p.created_at,
	p.first_name, p.last_name, p.nickname, p.avatar, p.group_name, p.images, p.viewers`

func scanPostView(s scanner) (*model.Post, error) {
	post := new(model.Post)
	var images, viewers string
	if err := s.Scan(
		&post.ID,
		&post.AuthorID,
		&post.Content,
		&post.Privacy,
		&post.GroupID,
		&post.CreatedAt,
		&post.AuthorFirstName,
		&post.AuthorLastName,
		&post.AuthorNickname,
		&post.AuthorAvatar,
		&post.GroupName,
		&images,
		&viewers,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(images), &post.Images); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(viewers), &post.Viewers); err != nil {
		return nil, err
	}
	return post, nil
}

func (r *PostRepository) GetPost(postID int64) (*model.Post, error) {
	post, err := scanPostView(r.QueryRow(
		`SELECT `+postViewColumns+`
		FROM post_view p
		WHERE p.id = ?`,
		postID,
	))
	if err != nil {
		return nil, notFound(err)
	}
	if post.CommentCount, err = r.comments.countPostComments(post.ID); err != nil {
		return nil, err
	}
	return post, nil
}

func (r *PostRepository) UpdatePostOwned(post *model.Post, ownerID int64, removedFileIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE posts SET content = ?, privacy = ? WHERE id = ? AND author_id = ?`,
		post.Content, post.Privacy, post.ID, ownerID,
	)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrNotFound
	}
	if len(removedFileIDs) > 0 {
		args := make([]any, 0, len(removedFileIDs)+1)
		args = append(args, post.ID)
		for _, fileID := range removedFileIDs {
			args = append(args, fileID)
		}
		if _, err := tx.Exec(
			`UPDATE files SET post_id = NULL WHERE post_id = ? AND id IN (`+placeholders(len(removedFileIDs))+`)`,
			args...,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *PostRepository) DeletePostOwned(postID, ownerID int64) error {
	result, err := r.db.Exec(`DELETE FROM posts WHERE id = ? AND author_id = ?`, postID, ownerID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePost removes one post regardless of who wrote it. Ownership and
// authorization are decided by the service layer; the database handles the
// related rows (comments and post_visibility cascade, files keep their rows
// with post_id cleared by ON DELETE SET NULL).
func (r *PostRepository) DeletePost(postID int64) error {
	result, err := r.db.Exec(`DELETE FROM posts WHERE id = ?`, postID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrNotFound
	}
	return nil
}

// postVisibleCondition is the single source of truth for "viewer can see this
// post row". Group posts bypass the privacy columns entirely: only members of
// the post's group can see them, no matter which privacy value the row
// carries. Normal (group-less) posts keep the author/public/followers/selected
// rules unchanged.
const postVisibleCondition = `(
	p.group_id IS NOT NULL AND EXISTS (
		SELECT 1 FROM group_members gm
		WHERE gm.group_id = p.group_id AND gm.user_id = ?
	)
	OR
	p.group_id IS NULL AND (
		p.author_id = ? OR p.privacy = ? OR
		(p.privacy = ? AND EXISTS (
			SELECT 1 FROM follow_requests f
			WHERE f.from_user_id = ? AND f.to_user_id = p.author_id AND f.status = ?
		)) OR
		(p.privacy = ? AND EXISTS (
			SELECT 1 FROM post_visibility v
			WHERE v.post_id = p.id AND v.user_id = ? AND EXISTS (
				SELECT 1 FROM follow_requests selected_follow
				WHERE selected_follow.from_user_id = v.user_id
				AND selected_follow.to_user_id = p.author_id
				AND selected_follow.status = ?
			)
		))
	)
)`

func postVisibleArgs(viewerID int64) []any {
	return []any{
		viewerID,
		viewerID, model.PostPublic, model.PostFollowersOnly, viewerID, model.FollowAccepted,
		model.PostSelected, viewerID, model.FollowAccepted,
	}
}

// ListVisiblePosts returns one page of the posts viewerID may see, newest
// first. With authorID set, only that user's posts (their profile); with 0,
// everybody's (the feed). The page starts after the post lastID (0: the first
// page). Ids only grow, so the newest post is the one with the biggest id.
func (r *PostRepository) ListVisiblePosts(viewerID, authorID, lastID int64) ([]*model.Post, error) {
	args := append([]any{
		authorID, authorID, authorID, authorID,
		viewerID, viewerID, model.FollowAccepted, model.PostSelected, viewerID,
		lastID, lastID,
	}, postVisibleArgs(viewerID)...)
	args = append(args, PageSize)
	rows, err := r.db.Query(`
		SELECT `+postViewColumns+`
		FROM post_view p
		WHERE (? = 0 OR p.group_id IS NULL) AND (? = 0 OR p.author_id = ?)
		AND (? != 0 OR (
			p.group_id IS NOT NULL OR p.author_id = ? OR
			EXISTS (
				SELECT 1 FROM follow_requests feed_follow
				WHERE feed_follow.from_user_id = ? AND feed_follow.to_user_id = p.author_id AND feed_follow.status = ?
			) OR
			(p.privacy = ? AND EXISTS (
				SELECT 1 FROM post_visibility feed_visibility
				WHERE feed_visibility.post_id = p.id AND feed_visibility.user_id = ?
			))
		))
		AND (? = 0 OR p.id < ?) AND `+postVisibleCondition+`
		ORDER BY p.id DESC
		LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]*model.Post, 0)
	for rows.Next() {
		post, err := scanPostView(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.enrichPosts(posts, viewerID)
}

// ListGroupPosts returns one page of the posts of one group, newest first,
// starting after the post lastID (0: the first page). The service
// layer checks group membership before calling this — the query itself is
// only reachable for authorized viewers.
func (r *PostRepository) ListGroupPosts(groupID, viewerID, lastID int64) ([]*model.Post, error) {
	rows, err := r.db.Query(`
		SELECT `+postViewColumns+`
		FROM post_view p
		WHERE p.group_id = ? AND (? = 0 OR p.id < ?)
		ORDER BY p.id DESC
		LIMIT ?`,
		groupID, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]*model.Post, 0)
	for rows.Next() {
		post, err := scanPostView(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return r.enrichPosts(posts, viewerID)
}

// enrichPosts attaches reaction summaries and comment counts to view-loaded posts.
func (r *PostRepository) enrichPosts(posts []*model.Post, viewerID int64) ([]*model.Post, error) {
	ids := make([]int64, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.ID)
	}
	counts, err := r.comments.CountPostComments(ids)
	if err != nil {
		return nil, err
	}
	for _, post := range posts {
		if err := r.LoadPostReactions(post, viewerID); err != nil {
			return nil, err
		}
		post.CommentCount = counts[post.ID]
	}
	return posts, nil
}

func (r *PostRepository) CanViewPost(viewerID, postID int64) (bool, error) {
	var visible bool
	args := append([]any{postID}, postVisibleArgs(viewerID)...)
	err := r.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM posts p WHERE p.id = ? AND `+postVisibleCondition+`)`,
		args...,
	).Scan(&visible)
	return visible, err
}

func (r *PostRepository) LoadPostReactions(post *model.Post, viewerID int64) error {
	summary, err := r.reactions.GetReactionSummary(model.ReactionTargetPost, post.ID, viewerID)
	if err != nil {
		return err
	}
	post.Likes = summary.Likes
	post.Dislikes = summary.Dislikes
	post.MyReaction = summary.MyReaction
	return nil
}
