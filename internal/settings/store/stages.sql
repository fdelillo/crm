-- Queries of the pipeline stages (002/DD-5, DD-6). Every one filters by the company (INV-04 of 001).

-- name: ListActiveStages :many
SELECT * FROM app.stages
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
ORDER BY position, id;

-- name: InsertStage :exec
INSERT INTO app.stages (id, tenant_id, name, type, color, position, created_at, updated_at)
VALUES (@id, @tenant_id, @name, @type, @color, @position, @now, @now);

-- name: UpdateStageFields :exec
UPDATE app.stages SET name = @name, type = @type, color = @color, updated_at = @now
WHERE tenant_id = @tenant_id AND id = @id AND deleted_at IS NULL;

-- name: SoftDeleteStage :exec
-- Deleting is logical and final (002/DD-6): the row stays for the history of later specs and its name is free again.
UPDATE app.stages SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE tenant_id = @tenant_id AND id = @id AND deleted_at IS NULL;

-- name: SetStagePositions :exec
-- Rewrites the positions of the active stages in one statement (002/DD-5): the position of an id is its place in the
-- array, from 1. Only the rows that change are written, so a stage that keeps its place keeps its updated_at.
UPDATE app.stages AS s SET position = v.position::integer, updated_at = @now::timestamptz
FROM unnest(@ids::uuid[]) WITH ORDINALITY AS v (id, position)
WHERE s.tenant_id = @tenant_id AND s.id = v.id AND s.deleted_at IS NULL AND s.position <> v.position;
