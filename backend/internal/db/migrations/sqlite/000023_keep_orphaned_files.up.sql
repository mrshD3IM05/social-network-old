DROP VIEW IF EXISTS post_view;

CREATE TABLE files_new (
    id            TEXT PRIMARY KEY,
    storage_path  TEXT NOT NULL UNIQUE,
    original_name TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size          INTEGER NOT NULL,
    owner_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    post_id       INTEGER REFERENCES posts(id) ON DELETE SET NULL,
    comment_id    INTEGER REFERENCES comments(id) ON DELETE SET NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    message_id    INTEGER REFERENCES messages(id) ON DELETE SET NULL
);

INSERT INTO files_new (id, storage_path, original_name, mime_type, size, owner_user_id, post_id, comment_id, created_at, message_id)
SELECT id, storage_path, original_name, mime_type, size, owner_user_id, post_id, comment_id, created_at, message_id
FROM files;

DROP TABLE files;
ALTER TABLE files_new RENAME TO files;

CREATE INDEX idx_files_owner ON files(owner_user_id);
CREATE INDEX idx_files_message ON files(message_id);

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