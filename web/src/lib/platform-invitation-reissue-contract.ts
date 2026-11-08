import type { PlatformRole } from "./platform-contract";
const PLATFORM_ROLES: PlatformRole[] = ["PLATFORM_OWNER", "OPERATIONS_ADMINISTRATOR", "BILLING_ADMINISTRATOR", "INTEGRATION_ADMINISTRATOR", "SUPPORT_ADMINISTRATOR", "SECURITY_AUDIT_ADMINISTRATOR"];
export type ReissuableInvitation = { first_name?:string;last_name?:string; id: string; email: string; role_key: PlatformRole; state: string; version: number; expires_at: string | null; delivery_state: string };
const uuid = /^[a-f\d]{8}-[a-f\d]{4}-[1-5][a-f\d]{3}-[89ab][a-f\d]{3}-[a-f\d]{12}$/i;
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
export function parseInvitationsForReissue(v: unknown): { invitations: ReissuableInvitation[]; next: string | null } | null {
 const root = record(v), data = record(root?.data), meta = record(root?.meta);
 if (!root || !data || !meta || !Array.isArray(data.invitations) || data.invitations.length > 20 || typeof meta.trace_id !== "string" || !(meta.next_cursor === null || typeof meta.next_cursor === "string" && /^[A-Za-z0-9_-]{1,512}$/.test(meta.next_cursor))) return null;
 const rows: ReissuableInvitation[] = [];
 for (const raw of data.invitations) { const row = record(raw);
  if (!row || !uuid.test(String(row.id)) || typeof row.email !== "string" || row.email.length < 3 || row.email.length > 254 || !PLATFORM_ROLES.includes(row.role_key as PlatformRole) || !["ACTIVE", "PENDING_APPROVAL", "ACCEPTED", "CANCELLED", "EXPIRED"].includes(String(row.state)) || !Number.isSafeInteger(row.version) || Number(row.version) < 1 || !(row.expires_at === null || typeof row.expires_at === "string" && Number.isFinite(Date.parse(row.expires_at))) || !["NOT_READY", "ACCEPTED_FOR_DELIVERY", "DELIVERY_UNCONFIRMED"].includes(String(row.delivery_state)) || rows.some((v) => v.id === row.id)) return null;
  rows.push(row as ReissuableInvitation);
 }
 return { invitations: rows, next: meta.next_cursor as string | null };
}
