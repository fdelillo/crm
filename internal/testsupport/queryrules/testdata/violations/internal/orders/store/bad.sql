-- name: ListUsers :many
SELECT id, email FROM app.users;

-- name: GetSession :one
SELECT id FROM app.sessions WHERE id = @id;

-- name: RenameTenant :exec
UPDATE app.tenants SET name = @name WHERE id = @id;

-- name: EnqueueEmail :exec
INSERT INTO app.outbox_messages (kind, template, recipient) VALUES (@kind, @template, @recipient);

-- name: WrongColumn :many
SELECT id FROM users WHERE created_at > @since;
