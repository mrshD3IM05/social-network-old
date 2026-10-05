-- post_view includes the attached image IDs so post reads can load the full
-- editable post state in one query.
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
    COALESCE(
        (SELECT json_group_array(f.id ORDER BY f.created_at, f.id)
         FROM files f
         WHERE f.post_id = p.id),
        '[]'
    ) AS images
FROM posts p
JOIN users u ON u.id = p.author_id;