package repository

import "sn-backend/internal/model"

const commentColumns = `
	c.id, c.post_id, c.author_id, c.content, c.created_at,
	u.first_name, u.last_name, u.nickname, u.avatar`

func scanComment(s scanner) (*model.Comment, error) {
	comment := new(model.Comment)
	if err := s.Scan(
		&comment.ID,
		&comment.PostID,
		&comment.AuthorID,
		&comment.Content,
		&comment.CreatedAt,
		&comment.AuthorFirstName,
		&comment.AuthorLastName,
		&comment.AuthorNickname,
		&comment.AuthorAvatar,
	); err != nil {
		return nil, err
	}
	return comment, nil
}

func (r *CommentRepository) CreateComment(comment *model.Comment) error {
	result, err := r.db.Exec(
		`INSERT INTO comments (post_id, author_id, content) VALUES (?, ?, ?)`,
		comment.PostID, comment.AuthorID, comment.Content,
	)
	if err != nil {
		return err
	}
	comment.ID, err = result.LastInsertId()
	return err
}

// GetComment loads one comment with its author fields.
func (r *CommentRepository) GetComment(id int64) (*model.Comment, error) {
	comment, err := scanComment(r.QueryRow(
		`SELECT `+commentColumns+`
		 FROM comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.id = ?`,
		id,
	))
	if err != nil {
		return nil, notFound(err)
	}
	return comment, nil
}

// ListPostComments returns one page of a post's comments, newest first,
// older than the comment lastID (0 for the newest page). The client shows them
// oldest at the top and asks for older ones with the id of the oldest it has,
// so a comment just written never shifts a page.
func (r *CommentRepository) ListPostComments(postID, lastID int64) ([]*model.Comment, error) {
	rows, err := r.db.Query(
		`SELECT `+commentColumns+`
		 FROM comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.post_id = ? AND (? = 0 OR c.id < ?)
		 ORDER BY c.id DESC
		 LIMIT ?`,
		postID, lastID, lastID, PageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]*model.Comment, 0)
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comment.Images, err = r.ListCommentFileIDs(comment.ID)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

// countPostComments returns the number of comments on a single post.
func (r *CommentRepository) countPostComments(postID int64) (int, error) {
	var count int
	err := r.QueryRow(`SELECT COUNT(*) FROM comments WHERE post_id = ?`, postID).Scan(&count)
	return count, err
}

// CountPostComments returns the number of comments per post ID for the given
// posts, so post lists can show a count without N+1 queries.
func (r *CommentRepository) CountPostComments(postIDs []int64) (map[int64]int, error) {
	counts := make(map[int64]int)
	if len(postIDs) == 0 {
		return counts, nil
	}
	rows, err := r.db.Query(
		`SELECT post_id, COUNT(*) FROM comments WHERE post_id IN (`+placeholders(len(postIDs))+`) GROUP BY post_id`,
		int64sToAny(postIDs)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var postID int64
		var count int
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, err
		}
		counts[postID] = count
	}
	return counts, rows.Err()
}

func (r *CommentRepository) ListCommentFileIDs(commentID int64) ([]string, error) {
	rows, err := r.db.Query(`SELECT id FROM files WHERE comment_id = ? ORDER BY created_at, id`, commentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateCommentOwned changes a comment the caller wrote.
func (r *CommentRepository) UpdateCommentOwned(commentID, authorID int64, content string) error {
	result, err := r.db.Exec(
		`UPDATE comments SET content = ? WHERE id = ? AND author_id = ?`,
		content, commentID, authorID,
	)
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

// DeleteCommentOwned removes a comment written by the caller, or any comment on
// a post the caller wrote. File rows are preserved with comment_id cleared by
// the foreign key.
func (r *CommentRepository) DeleteCommentOwned(commentID, userID int64) error {
	result, err := r.db.Exec(
		`DELETE FROM comments WHERE id = ? AND (
			author_id = ?
			OR EXISTS (SELECT 1 FROM posts p WHERE p.id = comments.post_id AND p.author_id = ?)
		)`,
		commentID, userID, userID,
	)
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
