export const PLATFORM_ROLES = [
  "PLATFORM_OWNER",
  "OPERATIONS_ADMINISTRATOR",
  "BILLING_ADMINISTRATOR",
  "INTEGRATION_ADMINISTRATOR",
  "SUPPORT_ADMINISTRATOR",
  "SECURITY_AUDIT_ADMINISTRATOR",
] as const;

export type PlatformRole = (typeof PLATFORM_ROLES)[number];
export const platformRoleLabels: Record<PlatformRole, string> = {
  PLATFORM_OWNER: "Platform Owner",
  OPERATIONS_ADMINISTRATOR: "Operations Administrator",
  BILLING_ADMINISTRATOR: "Billing Administrator",
  INTEGRATION_ADMINISTRATOR: "Integration Administrator",
  SUPPORT_ADMINISTRATOR: "Support Administrator",
  SECURITY_AUDIT_ADMINISTRATOR: "Security and Audit Administrator",
};
export type PlatformAdministrator = {
  user_id: string;
  user_version: number;
  platform_context_generation: number;
  email: string;
  state: "ACTIVE" | "DISABLED";
  totp_enrolled: boolean;
  assignments: Array<{ id: string; version: number; role_key: PlatformRole; state: "ACTIVE" | "REVOKED" }>;
  effective_owner: boolean;
};
export type PlatformRoleAssignment = {
  id: string;
  version: number;
  role_key: PlatformRole;
  state: "ACTIVE" | "REVOKED";
  assigned_at: string;
  revoked_at: string | null;
  reason: string;
  revocation_reason: string | null;
};
export type PlatformAssignmentHistory = { assignments: PlatformRoleAssignment[]; next_cursor: string | null };
export type PlatformAdministratorChange = {
  id: string; action: "ASSIGN_ROLE" | "REVOKE_ROLE" | "DISABLE_IDENTITY" | "REACTIVATE_IDENTITY" | "ACTIVATE_OWNER_INVITATION";
  target_user_id: string | null; role_key: PlatformRole | null; reason: string;
  state: "PENDING_APPROVAL" | "EXECUTED" | "REJECTED" | "EXPIRED";
  version: number; maker_id: string; created_at: string; expires_at: string | null;
};
export type PlatformChangePage = { changes: PlatformAdministratorChange[]; next_cursor: string | null };
export type PlatformInvitationReceipt = { id: string; state: "ACTIVE" | "PENDING_APPROVAL"; delivery_state: "NOT_READY" | "ACCEPTED_FOR_DELIVERY" | "DELIVERY_UNCONFIRMED" };
export type PlatformAdministratorList = {
  administrators: PlatformAdministrator[];
  effective_owner_count: number;
  continuity_warning: boolean;
  next_cursor: string | null;
};

function object(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}
function exact(value: Record<string, unknown>, keys: string[]): boolean {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
function instant(value: unknown): value is string {
  return typeof value === "string" && value.length <= 40 && Number.isFinite(Date.parse(value));
}
function optionalUUID(value: unknown): boolean { return value === null || typeof value === "string" && uuid.test(value); }
function role(value: unknown): value is PlatformRole { return typeof value === "string" && PLATFORM_ROLES.includes(value as PlatformRole); }

export function parsePlatformInvitationReceipt(value: unknown): PlatformInvitationReceipt | null {
  const root = object(value), data = object(root?.data), meta = object(root?.meta);
  if (!root || !data || !meta || !exact(root, ["data", "meta"]) || !exact(data, ["id", "email", "role_key", "state", "version", "created_at", "expires_at", "delivery_state", "governance_change_id"]) ||
      !uuid.test(String(data.id)) || typeof data.email !== "string" || !role(data.role_key) || !Number.isSafeInteger(data.version) || Number(data.version) < 1 || !instant(data.created_at) ||
      !(data.expires_at === null || instant(data.expires_at)) || !optionalUUID(data.governance_change_id) ||
      !["ACTIVE", "PENDING_APPROVAL"].includes(String(data.state)) || !["NOT_READY", "ACCEPTED_FOR_DELIVERY", "DELIVERY_UNCONFIRMED"].includes(String(data.delivery_state)) ||
      !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" || meta.next_cursor !== null) return null;
  return { id: data.id as string, state: data.state as PlatformInvitationReceipt["state"], delivery_state: data.delivery_state as PlatformInvitationReceipt["delivery_state"] };
}

export function parsePlatformAdministratorList(value: unknown): PlatformAdministratorList | null {
  const root = object(value);
  const data = object(root?.data);
  const meta = object(root?.meta);
  if (!root || !data || !meta || !exact(root, ["data", "meta"]) ||
      !exact(data, ["administrators", "effective_owner_count", "continuity_warning"]) ||
      !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" ||
      !(meta.next_cursor === null || typeof meta.next_cursor === "string" && meta.next_cursor.length <= 512) ||
      !Array.isArray(data.administrators) || data.administrators.length > 100 ||
      !Number.isInteger(data.effective_owner_count) || Number(data.effective_owner_count) < 1 ||
      typeof data.continuity_warning !== "boolean") return null;

  const administrators: PlatformAdministrator[] = [];
  const seen = new Set<string>();
  for (const raw of data.administrators) {
    const administrator = parseAdministratorItem(raw);
    if (!administrator || seen.has(administrator.user_id)) return null;
    seen.add(administrator.user_id);
    administrators.push(administrator);
  }
  return { administrators, effective_owner_count: Number(data.effective_owner_count), continuity_warning: data.continuity_warning, next_cursor: meta.next_cursor as string | null };
}

export function parsePlatformAdministratorDetail(value: unknown): PlatformAdministrator | null {
  const root = object(value);
  const meta = object(root?.meta);
  if (!root || !meta || !exact(root, ["data", "meta"]) || !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" || meta.next_cursor !== null) return null;
  return parseAdministratorItem(root.data);
}

export function parsePlatformAssignmentHistory(value: unknown, userId: string): PlatformAssignmentHistory | null {
  const root = object(value);
  const data = object(root?.data);
  const meta = object(root?.meta);
  if (!root || !data || !meta || !exact(root, ["data", "meta"]) || !exact(data, ["user_id", "assignments"]) ||
      !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" ||
      !(meta.next_cursor === null || typeof meta.next_cursor === "string" && meta.next_cursor.length <= 512) ||
      typeof data.user_id !== "string" || !uuid.test(data.user_id) || data.user_id.toLowerCase() !== userId.toLowerCase() ||
      !Array.isArray(data.assignments) || data.assignments.length > 100) return null;
  const assignments: PlatformRoleAssignment[] = [];
  const ids = new Set<string>();
  for (const raw of data.assignments) {
    const assignment = parseRoleAssignment(raw);
    if (!assignment || ids.has(assignment.id)) return null;
    ids.add(assignment.id);
    assignments.push(assignment);
  }
  return { assignments, next_cursor: meta.next_cursor as string | null };
}

export function parsePlatformAdministratorChange(value: unknown): PlatformAdministratorChange | null {
  const root = object(value);
  const meta = object(root?.meta);
  if (!root || !meta || !exact(root, ["data", "meta"]) || !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" || meta.next_cursor !== null) return null;
  return parseChange(root.data);
}

export function parsePlatformChangePage(value: unknown): PlatformChangePage | null {
  const root = object(value);
  const data = object(root?.data);
  const meta = object(root?.meta);
  if (!root || !data || !meta || !exact(root, ["data", "meta"]) || !exact(data, ["changes"]) ||
      !exact(meta, ["trace_id", "next_cursor"]) || typeof meta.trace_id !== "string" ||
      !(meta.next_cursor === null || typeof meta.next_cursor === "string" && meta.next_cursor.length <= 512) ||
      !Array.isArray(data.changes) || data.changes.length > 100) return null;
  const changes: PlatformAdministratorChange[] = [];
  const ids = new Set<string>();
  for (const raw of data.changes) {
    const change = parseChange(raw);
    if (!change || ids.has(change.id)) return null;
    ids.add(change.id);
    changes.push(change);
  }
  return { changes, next_cursor: meta.next_cursor as string | null };
}

function parseChange(raw: unknown): PlatformAdministratorChange | null {
  const v = object(raw);
  if (!v || !exact(v, ["id", "action", "target_user_id", "invitation_id", "role_key", "assignment_id", "expected_user_version", "expected_target_context_generation", "expected_assignment_version", "reason", "decision_reason", "state", "version", "maker_id", "checker_id", "created_at", "expires_at", "decided_at", "sessions_revoked"]) ||
      typeof v.id !== "string" || !uuid.test(v.id) ||
      !["ASSIGN_ROLE", "REVOKE_ROLE", "DISABLE_IDENTITY", "REACTIVATE_IDENTITY", "ACTIVATE_OWNER_INVITATION"].includes(String(v.action)) ||
      !optionalUUID(v.target_user_id) || !optionalUUID(v.invitation_id) || !optionalUUID(v.assignment_id) ||
      !(v.role_key === null || role(v.role_key)) ||
      ![v.expected_user_version, v.expected_target_context_generation, v.expected_assignment_version].every((item) => item === null || Number.isSafeInteger(item) && Number(item) > 0) ||
      typeof v.reason !== "string" || v.reason.length < 1 || v.reason.length > 2000 ||
      !(v.decision_reason === null || typeof v.decision_reason === "string" && v.decision_reason.length > 0 && v.decision_reason.length <= 2000) ||
      !["PENDING_APPROVAL", "EXECUTED", "REJECTED", "EXPIRED"].includes(String(v.state)) ||
      !Number.isSafeInteger(v.version) || Number(v.version) < 1 ||
      typeof v.maker_id !== "string" || !uuid.test(v.maker_id) || !optionalUUID(v.checker_id) ||
      !instant(v.created_at) || !(v.expires_at === null || instant(v.expires_at)) || !(v.decided_at === null || instant(v.decided_at)) ||
      typeof v.sessions_revoked !== "boolean") return null;
  return { id: v.id, action: v.action as PlatformAdministratorChange["action"], target_user_id: v.target_user_id as string | null,
    role_key: v.role_key as PlatformRole | null, reason: v.reason, state: v.state as PlatformAdministratorChange["state"],
    version: Number(v.version), maker_id: v.maker_id, created_at: v.created_at, expires_at: v.expires_at as string | null };
}

function parseAdministratorItem(raw: unknown): PlatformAdministrator | null {
    const item = object(raw);
    if (!item || !exact(item, ["user_id", "email", "state", "user_version", "platform_context_generation", "totp_enrolled", "assignments", "effective_owner", "created_at", "updated_at"]) ||
        typeof item.user_id !== "string" || !uuid.test(item.user_id) ||
        typeof item.email !== "string" || item.email.length < 3 || item.email.length > 254 ||
        (item.state !== "ACTIVE" && item.state !== "DISABLED") || !Number.isInteger(item.user_version) || Number(item.user_version) < 1 ||
        !Number.isInteger(item.platform_context_generation) || Number(item.platform_context_generation) < 1 ||
        typeof item.totp_enrolled !== "boolean" || typeof item.effective_owner !== "boolean" ||
        !instant(item.created_at) || !instant(item.updated_at) || !Array.isArray(item.assignments) || item.assignments.length > 6) return null;
    const assignments: PlatformAdministrator["assignments"] = [];
    const assignmentIDs = new Set<string>();
    for (const rawAssignment of item.assignments) {
      const assignment = parseRoleAssignment(rawAssignment);
      if (!assignment || assignmentIDs.has(assignment.id)) return null;
      assignmentIDs.add(assignment.id);
      assignments.push({ id: assignment.id, version: assignment.version, role_key: assignment.role_key, state: assignment.state });
    }
    return { user_id: item.user_id, user_version: Number(item.user_version), platform_context_generation: Number(item.platform_context_generation), email: item.email, state: item.state, totp_enrolled: item.totp_enrolled, assignments, effective_owner: item.effective_owner };
}

function parseRoleAssignment(raw: unknown): PlatformRoleAssignment | null {
  const assignment = object(raw);
  if (!assignment || !exact(assignment, ["id", "role_key", "state", "version", "assigned_by", "assigned_at", "revoked_by", "revoked_at", "reason", "revocation_reason"]) ||
      typeof assignment.id !== "string" || !uuid.test(assignment.id) || !role(assignment.role_key) ||
      (assignment.state !== "ACTIVE" && assignment.state !== "REVOKED") || !Number.isInteger(assignment.version) || Number(assignment.version) < 1 ||
      !optionalUUID(assignment.assigned_by) || !instant(assignment.assigned_at) || !optionalUUID(assignment.revoked_by) ||
      !(assignment.revoked_at === null || instant(assignment.revoked_at)) || typeof assignment.reason !== "string" || assignment.reason.length < 1 || assignment.reason.length > 2000 ||
      !(assignment.revocation_reason === null || typeof assignment.revocation_reason === "string" && assignment.revocation_reason.length >= 1 && assignment.revocation_reason.length <= 2000)) return null;
  return {
    id: assignment.id, version: Number(assignment.version), role_key: assignment.role_key, state: assignment.state, assigned_at: assignment.assigned_at,
    revoked_at: assignment.revoked_at, reason: assignment.reason, revocation_reason: assignment.revocation_reason,
  };
}
