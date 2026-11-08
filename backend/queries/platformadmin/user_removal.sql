-- name: RemovePlatformAdministratorProfile :execrows
UPDATE platform_administration.administrator_profiles SET removed_at=sqlc.arg(at),updated_at=sqlc.arg(at),version=version+1,context_generation=context_generation+1 WHERE user_id=sqlc.arg(user_id) AND version=sqlc.arg(expected_version) AND removed_at IS NULL;
