package repository

import "sn-backend/internal/model"

func (r *ReactionRepository) GetReaction(targetType string, targetID, userID int64) (*model.Reaction, error) {
	reaction := new(model.Reaction)
	err := r.QueryRow(
		`SELECT id, target_type, target_id, user_id, reaction, created_at
		 FROM reactions WHERE target_type = ? AND target_id = ? AND user_id = ?`,
		targetType, targetID, userID,
	).Scan(
		&reaction.ID,
		&reaction.TargetType,
		&reaction.TargetID,
		&reaction.UserID,
		&reaction.Reaction,
		&reaction.CreatedAt,
	)
	if err != nil {
		return nil, notFound(err)
	}
	return reaction, nil
}

func (r *ReactionRepository) SetReaction(targetType string, targetID, userID int64, reaction string) error {
	_, err := r.db.Exec(
		`INSERT INTO reactions (target_type, target_id, user_id, reaction)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (target_type, target_id, user_id)
		 DO UPDATE SET reaction = excluded.reaction, created_at = CURRENT_TIMESTAMP`,
		targetType, targetID, userID, reaction,
	)
	return err
}

func (r *ReactionRepository) DeleteReaction(targetType string, targetID, userID int64) error {
	_, err := r.db.Exec(
		`DELETE FROM reactions WHERE target_type = ? AND target_id = ? AND user_id = ?`,
		targetType, targetID, userID,
	)
	return err
}

func (r *ReactionRepository) GetReactionSummary(targetType string, targetID, viewerID int64) (*model.ReactionSummary, error) {
	summary := new(model.ReactionSummary)
	var mine *string
	err := r.QueryRow(
		`SELECT
			COUNT(CASE WHEN reaction = ? THEN 1 END),
			COUNT(CASE WHEN reaction = ? THEN 1 END),
			MAX(CASE WHEN user_id = ? THEN reaction END)
		 FROM reactions WHERE target_type = ? AND target_id = ?`,
		model.ReactionLike, model.ReactionDislike, viewerID, targetType, targetID,
	).Scan(&summary.Likes, &summary.Dislikes, &mine)
	if err != nil {
		return nil, err
	}
	if mine != nil {
		summary.MyReaction = *mine
	}
	return summary, nil
}
