import {
  ACCOUNT_TYPES,
  ASSURANCE_LEVELS,
  PORTAL_KEYS,
  PORTAL_PATHS,
  REQUIRED_ACTIONS,
  type MePortalResponse,
  type MembershipDirectoryResponse,
  type PortalDestination,
  type SessionBootstrapResponse,
  type SessionInventoryResponse,
} from "@/contracts/auth.generated";

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function exactKeys(value: Record<string, unknown>, keys: string[]): boolean {
  const actual = Object.keys(value).sort();
  return actual.length === keys.length && actual.every((key, index) => key === [...keys].sort()[index]);
}

function member<T extends readonly string[]>(values: T, value: unknown): value is T[number] {
  return typeof value === "string" && values.includes(value as T[number]);
}

function dateTime(value: unknown): value is string {
  return typeof value === "string" && value.length > 0 && Number.isFinite(Date.parse(value));
}

function uniqueStrings(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === "string" && entry.length > 0) &&
    new Set(value).size === value.length;
}

export function parseSessionBootstrap(value: unknown): SessionBootstrapResponse | null {
  const root = record(value);
  const data = record(root?.data);
  const meta = record(root?.meta);
  if (
    !root ||
    !data ||
    !meta ||
    !exactKeys(root, ["data", "meta"]) ||
    !exactKeys(data, ["csrf_token"]) ||
    !exactKeys(meta, ["trace_id"]) ||
    typeof data.csrf_token !== "string" ||
    data.csrf_token.length !== 43 ||
    typeof meta.trace_id !== "string"
  ) {
    return null;
  }
  return value as SessionBootstrapResponse;
}

function parsePortal(value: unknown): PortalDestination | null | undefined {
  if (value === null) return null;
  const portal = record(value);
  if (
    !portal ||
    !exactKeys(portal, ["key", "path"]) ||
    !member(PORTAL_KEYS, portal.key) ||
    !member(PORTAL_PATHS, portal.path)
  ) {
    return undefined;
  }
  const expected = {
    TENDERPRENEUR: "/tenderpreneur",
    PROCUREMENT: "/procurement",
    PLATFORM_ADMIN: "/platform",
  } as const;
  return expected[portal.key] === portal.path ? (portal as unknown as PortalDestination) : undefined;
}

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function parseMembershipDirectory(value: unknown): MembershipDirectoryResponse | null {
  const root = record(value);
  const data = record(root?.data);
  const meta = record(root?.meta);
  if (!root || !data || !meta || !exactKeys(root, ["data", "meta"]) || !exactKeys(data, ["memberships"]) || !Array.isArray(data.memberships)) {
    return null;
  }

  const membershipIDs = new Set<string>();
  const tenantIDs = new Set<string>();
  for (const entry of data.memberships) {
    const membership = record(entry);
    if (
      !membership ||
      !exactKeys(membership, ["membership_id", "tenant_id", "tenant_type", "tenant_name", "context_generation", "capabilities"]) ||
      typeof membership.membership_id !== "string" ||
      !uuidPattern.test(membership.membership_id) ||
      typeof membership.tenant_id !== "string" ||
      !uuidPattern.test(membership.tenant_id) ||
      membership.tenant_type !== "TENDERPRENEUR" ||
      typeof membership.tenant_name !== "string" ||
      membership.tenant_name.length === 0 ||
      !Number.isInteger(membership.context_generation) ||
      Number(membership.context_generation) < 1 ||
      !uniqueStrings(membership.capabilities) ||
      membershipIDs.has(membership.membership_id) ||
      tenantIDs.has(membership.tenant_id)
    ) {
      return null;
    }
    membershipIDs.add(membership.membership_id);
    tenantIDs.add(membership.tenant_id);
  }
  return value as MembershipDirectoryResponse;
}

export function parseSessionInventory(value: unknown): SessionInventoryResponse | null {
  const root = record(value);
  const data = record(root?.data);
  const meta = record(root?.meta);
  if (!root || !data || !meta || !exactKeys(root, ["data", "meta"]) || !exactKeys(meta, ["trace_id"]) || typeof meta.trace_id !== "string" ||
    !exactKeys(data, ["active_session_count", "other_session_count", "sessions", "current_session_logout_available", "other_session_revocation_available", "revoke_all_available", "revocation_policy_configured", "device_metadata_collected", "blockers", "observed_at"]) ||
    !Number.isInteger(data.active_session_count) || Number(data.active_session_count) < 1 || Number(data.active_session_count) > 100 ||
    !Number.isInteger(data.other_session_count) || Number(data.other_session_count) < 0 || Number(data.other_session_count) > 99 ||
    !Array.isArray(data.sessions) || data.sessions.length !== data.active_session_count ||
    typeof data.current_session_logout_available !== "boolean" || typeof data.other_session_revocation_available !== "boolean" ||
    typeof data.revoke_all_available !== "boolean" || typeof data.revocation_policy_configured !== "boolean" ||
    data.device_metadata_collected !== false || !uniqueStrings(data.blockers) || !dateTime(data.observed_at)) return null;
  const ids = new Set<string>();
  let current = 0;
  for (const entry of data.sessions) {
    const item = record(entry);
    if (!item || !exactKeys(item, ["session_id", "rotated_from_session_id", "current", "created_at", "last_seen_at", "idle_expires_at", "absolute_expires_at"]) ||
      typeof item.session_id !== "string" || !uuidPattern.test(item.session_id) || ids.has(item.session_id) ||
      !(item.rotated_from_session_id === null || typeof item.rotated_from_session_id === "string" && uuidPattern.test(item.rotated_from_session_id)) ||
      typeof item.current !== "boolean" || !dateTime(item.created_at) || !dateTime(item.last_seen_at) || !dateTime(item.idle_expires_at) || !dateTime(item.absolute_expires_at)) return null;
    ids.add(item.session_id);
    if (item.current) current++;
  }
  if (current !== 1 || data.other_session_count !== data.sessions.length - 1 || data.other_session_revocation_available !== (data.other_session_count > 0)) return null;
  return value as SessionInventoryResponse;
}

export function parseMePortal(value: unknown): MePortalResponse | null {
  const root = record(value);
  const data = record(root?.data);
  const meta = record(root?.meta);
  const session = record(data?.session);
  const portal = parsePortal(data?.portal);
  if (
    !root ||
    !data ||
    !meta ||
    !session ||
    portal === undefined ||
    !exactKeys(root, ["data", "meta"]) ||
    !exactKeys(data, ["account_type", "portal", "platform_context_generation", "capabilities", "session", "required_actions"]) ||
    !exactKeys(session, ["assurance_level", "idle_expires_at", "absolute_expires_at"]) ||
    !exactKeys(meta, ["trace_id"]) ||
    !member(ACCOUNT_TYPES, data.account_type) ||
    !member(ASSURANCE_LEVELS, session.assurance_level) ||
    !dateTime(session.idle_expires_at) ||
    !dateTime(session.absolute_expires_at) ||
    !uniqueStrings(data.capabilities) ||
    !Array.isArray(data.required_actions) ||
    !data.required_actions.every((action) => member(REQUIRED_ACTIONS, action)) ||
    new Set(data.required_actions).size !== data.required_actions.length ||
    !(data.platform_context_generation === null ||
      (Number.isInteger(data.platform_context_generation) && Number(data.platform_context_generation) >= 1)) ||
    typeof meta.trace_id !== "string"
  ) {
    return null;
  }
  if (portal && portal.key !== data.account_type) return null;

  const actions = data.required_actions as string[];
  const noPlatformFacts = data.platform_context_generation === null && data.capabilities.length === 0;
  switch (data.account_type) {
    case "UNASSIGNED":
      if (portal !== null || !noPlatformFacts || actions.length !== 1 || actions[0] !== "SELECT_ACCOUNT_TYPE") return null;
      break;
    case "TENDERPRENEUR":
      if (!noPlatformFacts) return null;
      if (portal === null) {
        if (actions.length !== 1 || actions[0] !== "ESTABLISH_TENDERPRENEUR_MEMBERSHIP") return null;
      } else if (portal.path !== "/tenderpreneur" || actions.length !== 0) return null;
      break;
    case "PROCUREMENT":
      if (portal?.path !== "/procurement" || !noPlatformFacts || actions.length !== 0) return null;
      break;
    case "PLATFORM_ADMIN":
      if (data.platform_context_generation === null || data.capabilities.length === 0) return null;
      if (portal === null) {
        if (actions.length !== 1 || (actions[0] !== "ENROLL_PLATFORM_MFA" && actions[0] !== "COMPLETE_PLATFORM_STEP_UP")) return null;
      } else if (portal.path !== "/platform" || actions.length !== 0) return null;
      break;
  }
  return value as MePortalResponse;
}
