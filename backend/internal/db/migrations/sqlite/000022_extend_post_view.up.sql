DROP VIEW IF EXISTS post_view;

CREATE VIEW post_view AS
SELECT
    p.id,
    p.author_id,
    p.content,
    p.privacy,
    p.group_id,
    p.created_at,
    u.first_name,
    u.last_name,
    u.nickname,
    u.avatar,
    CASE WHEN p.group_id IS NOT NULL THEN g.title ELSE '' END AS group_name,
    COALESCE(
        (SELECT json_group_array(f.id ORDER BY f.created_at, f.id)
         FROM files f
         WHERE f.post_id = p.id),
        '[]'
    ) AS images,
    CASE WHEN p.group_id IS NULL AND p.privacy = 'almost_private' THEN COALESCE(
        (SELECT json_group_array(fr.from_user_id ORDER BY fr.from_user_id)
         FROM follow_requests fr
         WHERE fr.to_user_id = p.author_id AND fr.status = 'accepted'),
        '[]'
    ) ELSE '[]' END AS viewers
FROM posts p
JOIN users u ON u.id = p.author_id
LEFT JOIN groups g ON g.id = p.group_id;