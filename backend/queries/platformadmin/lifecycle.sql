-- name: LockPlatformGovernanceState :one
SELECT bootstrap_consumed, bootstrap_owner_id, bootstrap_consumed_at,
       first_additional_owner_exception_consumed, effective_owner_generation,
       version, updated_at
FROM platform_administration.governance_state
WHERE singleton = true
FOR UPDATE;

-- name: PlatformAdministratorProfileExists :one
SELECT EXISTS (
    SELECT 1 FROM platform_administration.administrator_profiles
    WHERE user_id = sqlc.arg(user_id)
);

-- name: CountPlatformOwnerAssignmentHistory :one
SELECT count(*)::bigint
FROM platform_administration.role_assignments
WHERE role_key = 'PLATFORM_OWNER';

-- name: InsertPlatformAdministratorProfile :exec
INSERT INTO platform_administration.administrator_profiles
(user_id, context_generation, version, created_at, updated_at)
VALUES (sqlc.arg(user_id), 1, 1, sqlc.arg(at), sqlc.arg(at));

-- name: InsertPlatformRoleAssignment :exec
INSERT INTO platform_administration.role_assignments
(id, user_id, role_key, state, source, assigned_by, assigned_at, reason, version)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(role_key), 'ACTIVE', sqlc.arg(source),
        sqlc.narg(assigned_by), sqlc.arg(assigned_at), sqlc.arg(reason), 1);

-- name: ConsumePlatformBootstrap :execrows
UPDATE platform_administration.governance_state
SET bootstrap_consumed = true, bootstrap_owner_id = sqlc.arg(owner_user_id),
    bootstrap_consumed_at = sqlc.arg(at), effective_owner_generation = 1,
    version = version + 1, updated_at = sqlc.arg(at)
WHERE singleton = true AND bootstrap_consumed = false AND version = sqlc.arg(expected_version);

-- name: GetPlatformAuthorityProfile :one
SELECT user_id, context_generation, version AS administrator_version
FROM platform_administration.administrator_profiles
WHERE user_id = sqlc.arg(user_id);

-- name: ListActivePlatformRoleKeys :many
SELECT role_key
FROM platform_administration.role_assignments
WHERE user_id = sqlc.arg(user_id) AND state = 'ACTIVE'
ORDER BY role_key;

-- name: ListActivePlatformOwnerIDs :many
SELECT user_id
FROM platform_administration.role_assignments
WHERE role_key = 'PLATFORM_OWNER' AND state = 'ACTIVE'
ORDER BY user_id;

-- name: ListActivePlatformOwnerIDsForUpdate :many
SELECT user_id
FROM platform_administration.role_assignments
WHERE role_key = 'PLATFORM_OWNER' AND state = 'ACTIVE'
ORDER BY user_id
FOR UPDATE;

-- name: GetPlatformCommand :one
SELECT request_hash, target_id, response_status
FROM platform_administration.commands
WHERE actor_user_id = sqlc.arg(actor_user_id)
  AND operation = sqlc.arg(operation)
  AND key_hash = sqlc.arg(key_hash);

-- name: ClaimPlatformCommand :execrows
INSERT INTO platform_administration.commands
(id, actor_user_id, operation, key_hash, request_hash, target_id, response_status, created_at, completed_at)
VALUES (sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.arg(operation), sqlc.arg(key_hash),
        sqlc.arg(request_hash), sqlc.narg(target_id), sqlc.arg(response_status), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (actor_user_id, operation, key_hash) DO NOTHING;

-- name: FindOpenPlatformOwnerInvitation :one
SELECT id, email, email_lookup, role_key, authorization_path, state, invited_by, reason, created_at,
       activated_at, expires_at, version
FROM platform_administration.administrator_invitations
WHERE role_key = 'PLATFORM_OWNER' AND state IN ('PENDING_APPROVAL','ACTIVE')
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: ExpirePendingChangesForOpenInvitationEmail :execrows
UPDATE platform_administration.administrator_changes AS c
SET state = 'EXPIRED', decided_at = sqlc.arg(at), version = c.version + 1
WHERE c.state = 'PENDING_APPROVAL'
  AND EXISTS (
      SELECT 1 FROM platform_administration.administrator_invitations AS i
      WHERE i.id = c.invitation_id AND i.email_lookup = sqlc.arg(email_lookup)
        AND i.state IN ('PENDING_APPROVAL','ACTIVE')
  );

-- name: ReplaceOpenPlatformInvitations :exec
UPDATE platform_administration.administrator_invitations
SET state = 'REPLACED', replaced_at = sqlc.arg(at), version = version + 1
WHERE email_lookup = sqlc.arg(email_lookup) AND state IN ('PENDING_APPROVAL','ACTIVE');

-- name: InsertPlatformInvitation :exec
INSERT INTO platform_administration.administrator_invitations
(id, email, email_lookup, role_key, authorization_path, state, token_hash, invited_by, reason,
 created_at, activated_at, expires_at, version)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(email_lookup), sqlc.arg(role_key), sqlc.arg(authorization_path), sqlc.arg(state),
        sqlc.narg(token_hash), sqlc.arg(invited_by), sqlc.arg(reason), sqlc.arg(created_at),
        sqlc.narg(activated_at), sqlc.narg(expires_at), 1);

-- name: GetPlatformInvitation :one
SELECT id, email, email_lookup, role_key, authorization_path, state, invited_by, reason, created_at, activated_at,
       expires_at, accepted_by, accepted_at, cancelled_at, replaced_at, version
FROM platform_administration.administrator_invitations
WHERE id = sqlc.arg(id);

-- name: GetPlatformInvitationChangeID :one
SELECT id
FROM platform_administration.administrator_changes
WHERE invitation_id = sqlc.arg(invitation_id)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: LockActivePlatformInvitationByToken :one
SELECT id, email, email_lookup, role_key, authorization_path, invited_by, reason, created_at, activated_at, expires_at, version
FROM platform_administration.administrator_invitations
WHERE token_hash = sqlc.arg(token_hash) AND state = 'ACTIVE' AND expires_at > sqlc.arg(observed_at)
FOR UPDATE;

-- name: ActivateApprovedOwnerInvitation :execrows
UPDATE platform_administration.administrator_invitations
SET state = 'ACTIVE', token_hash = sqlc.arg(token_hash), activated_at = sqlc.arg(at),
    expires_at = sqlc.arg(at) + interval '72 hours', version = version + 1
WHERE id = sqlc.arg(id) AND role_key = 'PLATFORM_OWNER' AND authorization_path = 'MAKER_CHECKER'
  AND state = 'PENDING_APPROVAL' AND version = sqlc.arg(expected_version);

-- name: LockCancelablePlatformInvitation :one
SELECT id, email, email_lookup, role_key, authorization_path, state, invited_by, reason, created_at,
       activated_at, expires_at, version
FROM platform_administration.administrator_invitations
WHERE id = sqlc.arg(id) AND state IN ('PENDING_APPROVAL','ACTIVE')
  AND version = sqlc.arg(expected_version)
FOR UPDATE;

-- name: CancelPlatformInvitation :execrows
UPDATE platform_administration.administrator_invitations
SET state = 'CANCELLED', cancelled_at = sqlc.arg(at), version = version + 1
WHERE id = sqlc.arg(id) AND state IN ('PENDING_APPROVAL','ACTIVE')
  AND version = sqlc.arg(expected_version);

-- name: ExpirePendingInvitationChangeOnCancellation :execrows
UPDATE platform_administration.administrator_changes
SET state = 'EXPIRED', decided_at = sqlc.arg(at), version = version + 1
WHERE invitation_id = sqlc.arg(invitation_id) AND state = 'PENDING_APPROVAL';

-- name: AcceptPlatformInvitation :execrows
UPDATE platform_administration.administrator_invitations
SET state = 'ACCEPTED', accepted_by = sqlc.arg(accepted_by), accepted_at = sqlc.arg(at), version = version + 1
WHERE id = sqlc.arg(id) AND state = 'ACTIVE' AND version = sqlc.arg(expected_version)
  AND expires_at > sqlc.arg(at);

-- name: ListPlatformInvitations :many
SELECT i.id, i.email, i.role_key, i.state, i.invited_by, i.reason, i.created_at, i.activated_at,
       i.expires_at, i.accepted_by, i.accepted_at, i.cancelled_at, i.replaced_at, i.version,
       (SELECT c.id FROM platform_administration.administrator_changes AS c
        WHERE c.invitation_id = i.id AND c.action = 'ACTIVATE_OWNER_INVITATION'
        ORDER BY c.created_at DESC, c.id DESC LIMIT 1) AS governance_change_id
FROM platform_administration.administrator_invitations AS i
WHERE i.state IN ('PENDING_APPROVAL','ACTIVE')
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (i.created_at, i.id) < (sqlc.narg(before_created_at), sqlc.narg(before_id)::uuid))
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(page_limit);

-- name: InsertPlatformAdministratorChange :exec
INSERT INTO platform_administration.administrator_changes
(id, action, target_user_id, invitation_id, role_key, assignment_id,
 expected_user_version, expected_target_context_generation, expected_assignment_version,
 reason, state, maker_id, created_at, expires_at, decided_at, version)
VALUES (sqlc.arg(id), sqlc.arg(action), sqlc.narg(target_user_id), sqlc.narg(invitation_id),
        sqlc.narg(role_key), sqlc.narg(assignment_id), sqlc.narg(expected_user_version),
        sqlc.narg(expected_target_context_generation), sqlc.narg(expected_assignment_version),
        sqlc.arg(reason), sqlc.arg(state), sqlc.arg(maker_id), sqlc.arg(created_at),
        sqlc.narg(expires_at), sqlc.narg(decided_at), 1);

-- name: LockPendingOwnerInvitationChange :one
SELECT c.id AS change_id, c.invitation_id, c.maker_id, c.reason, c.expires_at,
       c.version AS change_version, i.id, i.email, i.email_lookup, i.role_key,
       i.authorization_path, i.state, i.invited_by, i.reason AS invitation_reason,
       i.created_at, i.activated_at, i.expires_at AS invitation_expires_at,
       i.version AS invitation_version
FROM platform_administration.administrator_changes AS c
JOIN platform_administration.administrator_invitations AS i ON i.id = c.invitation_id
WHERE c.id = sqlc.arg(change_id) AND c.action = 'ACTIVATE_OWNER_INVITATION'
  AND c.role_key = 'PLATFORM_OWNER' AND c.state = 'PENDING_APPROVAL'
  AND c.expires_at > sqlc.arg(observed_at)
  AND i.role_key = 'PLATFORM_OWNER' AND i.authorization_path = 'MAKER_CHECKER'
  AND i.state = 'PENDING_APPROVAL'
FOR UPDATE OF c, i;

-- name: GetPlatformAdministratorChangeState :one
SELECT target_user_id, action, state
FROM platform_administration.administrator_changes
WHERE id = sqlc.arg(change_id);

-- name: LockPendingPlatformOwnerIdentityStateChange :one
SELECT id, action, target_user_id, expected_user_version,
       expected_target_context_generation, reason, maker_id, expires_at, version
FROM platform_administration.administrator_changes
WHERE id = sqlc.arg(change_id)
  AND action IN ('DISABLE_IDENTITY','REACTIVATE_IDENTITY')
  AND state = 'PENDING_APPROVAL' AND expires_at > sqlc.arg(observed_at)
FOR UPDATE;

-- name: LockPendingPlatformOwnerRoleChange :one
SELECT id, action, target_user_id, role_key, assignment_id,
       expected_user_version, expected_target_context_generation,
       expected_assignment_version, reason, maker_id, created_at, expires_at, version
FROM platform_administration.administrator_changes
WHERE id = sqlc.arg(change_id) AND action IN ('ASSIGN_ROLE','REVOKE_ROLE')
  AND role_key = 'PLATFORM_OWNER' AND state = 'PENDING_APPROVAL'
  AND expires_at > sqlc.arg(observed_at)
FOR UPDATE;

-- name: LockPendingPlatformAdministratorChange :one
SELECT id, action, target_user_id, invitation_id, role_key, assignment_id,
       expected_user_version, expected_target_context_generation, expected_assignment_version,
       reason, maker_id, created_at, expires_at, version
FROM platform_administration.administrator_changes
WHERE id = sqlc.arg(id) AND state = 'PENDING_APPROVAL' AND expires_at > sqlc.arg(observed_at)
FOR UPDATE;

-- name: DecidePlatformAdministratorChange :execrows
UPDATE platform_administration.administrator_changes
SET state = sqlc.arg(state), decision_reason = sqlc.arg(decision_reason),
    checker_id = sqlc.arg(checker_id), decided_at = sqlc.arg(at), version = version + 1
WHERE id = sqlc.arg(id) AND state = 'PENDING_APPROVAL' AND version = sqlc.arg(expected_version)
  AND expires_at > sqlc.arg(at) AND maker_id <> sqlc.arg(checker_id);

-- name: RejectPlatformOwnerInvitation :execrows
UPDATE platform_administration.administrator_invitations
SET state = 'EXPIRED', expires_at = sqlc.arg(at), version = version + 1
WHERE id = sqlc.arg(id) AND state = 'PENDING_APPROVAL'
  AND authorization_path = 'MAKER_CHECKER' AND version = sqlc.arg(expected_version);

-- name: ExpirePlatformAdministratorChanges :exec
UPDATE platform_administration.administrator_changes
SET state = 'EXPIRED', decided_at = sqlc.arg(at), version = version + 1
WHERE state = 'PENDING_APPROVAL' AND expires_at <= sqlc.arg(at);

-- name: GetPlatformAdministratorChange :one
SELECT id, action, target_user_id, invitation_id, role_key, assignment_id,
       expected_user_version, expected_target_context_generation, expected_assignment_version,
       reason, decision_reason, state, maker_id, checker_id, created_at, expires_at, decided_at, version
FROM platform_administration.administrator_changes
WHERE id = sqlc.arg(id);

-- name: ListPlatformAdministratorChanges :many
SELECT id, action, target_user_id, invitation_id, role_key, assignment_id,
       expected_user_version, expected_target_context_generation, expected_assignment_version,
       reason, decision_reason, state, maker_id, checker_id, created_at, expires_at, decided_at, version
FROM platform_administration.administrator_changes
WHERE (sqlc.narg(filter_state)::text IS NULL OR
       (CASE WHEN state = 'PENDING_APPROVAL' AND expires_at <= sqlc.arg(observed_at)
             THEN 'EXPIRED' ELSE state END) = sqlc.narg(filter_state))
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(before_created_at), sqlc.narg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: LockPlatformAdministratorProfile :one
SELECT user_id, context_generation, version
FROM platform_administration.administrator_profiles
WHERE user_id = sqlc.arg(user_id)
FOR UPDATE;

-- name: AdvancePlatformAdministratorProfile :execrows
UPDATE platform_administration.administrator_profiles
SET context_generation = context_generation + 1, version = version + 1, updated_at = sqlc.arg(at)
WHERE user_id = sqlc.arg(user_id) AND context_generation = sqlc.arg(expected_context_generation)
  AND version = sqlc.arg(expected_version);

-- name: PlatformActiveRoleExists :one
SELECT EXISTS (
    SELECT 1 FROM platform_administration.role_assignments
    WHERE user_id = sqlc.arg(user_id) AND role_key = sqlc.arg(role_key) AND state = 'ACTIVE'
);

-- name: LockActivePlatformRoleAssignment :one
SELECT id, user_id, role_key, state, source, assigned_by, assigned_at, reason, version
FROM platform_administration.role_assignments
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND state = 'ACTIVE'
FOR UPDATE;

-- name: RevokePlatformRoleAssignment :execrows
UPDATE platform_administration.role_assignments
SET state = 'REVOKED', revoked_by = sqlc.arg(revoked_by), revoked_at = sqlc.arg(at),
    revocation_reason = sqlc.arg(revocation_reason), version = version + 1
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND state = 'ACTIVE'
  AND version = sqlc.arg(expected_version);

-- name: AdvancePlatformGovernanceOwnerGeneration :execrows
UPDATE platform_administration.governance_state
SET effective_owner_generation = effective_owner_generation + 1,
    version = version + 1, updated_at = sqlc.arg(at)
WHERE singleton = true AND version = sqlc.arg(expected_version);

-- name: ConsumeFirstAdditionalOwnerException :execrows
UPDATE platform_administration.governance_state
SET first_additional_owner_exception_consumed = true,
    effective_owner_generation = effective_owner_generation + 1,
    version = version + 1, updated_at = sqlc.arg(at)
WHERE singleton = true AND first_additional_owner_exception_consumed = false
  AND version = sqlc.arg(expected_version);

-- name: ListPlatformAdministratorProfiles :many
SELECT user_id, context_generation, version, created_at, updated_at
FROM platform_administration.administrator_profiles AS p
WHERE (sqlc.narg(filter_user_id)::uuid IS NULL OR p.user_id = sqlc.narg(filter_user_id)::uuid)
  AND (sqlc.narg(filter_role_key)::text IS NULL OR EXISTS (
       SELECT 1 FROM platform_administration.role_assignments AS r
       WHERE r.user_id = p.user_id AND r.role_key = sqlc.narg(filter_role_key) AND r.state = 'ACTIVE'))
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (p.created_at, p.user_id) < (sqlc.narg(before_created_at), sqlc.narg(before_user_id)::uuid))
ORDER BY p.created_at DESC, p.user_id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetPlatformAdministratorProfile :one
SELECT user_id, context_generation, version, created_at, updated_at
FROM platform_administration.administrator_profiles
WHERE user_id = sqlc.arg(user_id);

-- name: ListLatestPlatformRoleAssignments :many
SELECT DISTINCT ON (role_key) id, user_id, role_key, state, source, assigned_by,
       assigned_at, revoked_by, revoked_at, reason, revocation_reason, version
FROM platform_administration.role_assignments
WHERE user_id = sqlc.arg(user_id)
ORDER BY role_key, assigned_at DESC, id DESC;

-- name: ListLatestPlatformRoleAssignmentsForUsers :many
SELECT DISTINCT ON (user_id, role_key) id, user_id, role_key, state, source, assigned_by,
       assigned_at, revoked_by, revoked_at, reason, revocation_reason, version
FROM platform_administration.role_assignments
WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY user_id, role_key, assigned_at DESC, id DESC;

-- name: ListPlatformRoleAssignmentHistory :many
SELECT id, user_id, role_key, state, source, assigned_by, assigned_at,
       revoked_by, revoked_at, reason, revocation_reason, version
FROM platform_administration.role_assignments
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(before_assigned_at)::timestamptz IS NULL
       OR (assigned_at, id) < (sqlc.narg(before_assigned_at), sqlc.narg(before_id)::uuid))
ORDER BY assigned_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: HasPlatformPasswordOwner :one
SELECT EXISTS(SELECT 1 FROM platform_administration.role_assignments r
 JOIN identity.users u ON u.id=r.user_id
 JOIN identity.authentication_identities a ON a.user_id=u.id
 WHERE r.role_key='PLATFORM_OWNER' AND r.state='ACTIVE'
 AND u.account_type='PLATFORM_ADMIN' AND u.disabled_at IS NULL AND u.email_verified_at IS NOT NULL
 AND EXISTS(SELECT 1 FROM identity.sessions s WHERE s.user_id=u.id AND s.auth_epoch=u.auth_epoch
   AND s.revoked_at IS NULL AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP')
   AND s.idle_expires_at>CURRENT_TIMESTAMP AND s.absolute_expires_at>CURRENT_TIMESTAMP));

