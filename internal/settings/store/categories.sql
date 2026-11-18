-- Queries of the income and expense categories (002/DD-8). Every one filters by the company (INV-04 of 001).

-- name: ListCategories :many
-- Income first; inside each type, the system categories and then the company's, by name (002/FR-003).
SELECT * FROM app.categories
WHERE tenant_id = @tenant_id
  AND (sqlc.narg(type)::text IS NULL OR type = sqlc.narg(type)::text)
  AND (@include_archived::boolean OR archived_at IS NULL)
ORDER BY CASE type WHEN 'income' THEN 0 ELSE 1 END, (system_key IS NULL), lower(name), id;

-- name: InsertCategory :exec
-- A name taken by an active category of the same type fails on categories_active_name_key (002/INV-6).
INSERT INTO app.categories (id, tenant_id, name, type, created_at, updated_at)
VALUES (@id, @tenant_id, @name, @type, @now, @now);

-- name: UpdateCategoryName :exec
UPDATE app.categories SET name = @name, updated_at = @now::timestamptz
WHERE tenant_id = @tenant_id AND id = @id;

-- name: SetCategoryArchived :exec
-- A NULL archived_at restores the category; restoring may fail on categories_active_name_key.
UPDATE app.categories SET archived_at = sqlc.narg(archived_at)::timestamptz, updated_at = @now::timestamptz
WHERE tenant_id = @tenant_id AND id = @id;
