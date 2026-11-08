// Kept locally so this strict contract can also run in Node's direct TS test runner.
const auditUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export type SupportCase = { id: string; tenant_id: string; title: string; description: string; state: "OPEN" | "RESOLVED"; assigned_to: string | null; created_by: string; version: number; created_at: string; updated_at: string };
export type SupportMessage = { id: string; actor_id: string; body: string; created_at: string };
const object = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null && !Array.isArray(v);
const exact = (v: Record<string, unknown>, names: string[]) => Object.keys(v).length === names.length && names.every((key) => key in v);
const timestamp = (v: unknown) => typeof v === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(v) && !Number.isNaN(Date.parse(v));
export function isSupportCase(v: unknown): v is SupportCase {
  return object(v) && exact(v, ["id", "tenant_id", "title", "description", "state", "assigned_to", "created_by", "version", "created_at", "updated_at"]) &&
    typeof v.id === "string" && auditUUID.test(v.id) && typeof v.tenant_id === "string" && auditUUID.test(v.tenant_id) &&
    typeof v.created_by === "string" && auditUUID.test(v.created_by) && (v.assigned_to === null || typeof v.assigned_to === "string" && auditUUID.test(v.assigned_to)) &&
    typeof v.title === "string" && v.title.length >= 3 && v.title.length <= 120 && typeof v.description === "string" && v.description.length >= 3 && v.description.length <= 2000 &&
    (v.state === "OPEN" || v.state === "RESOLVED") && Number.isSafeInteger(v.version) && (v.version as number) > 0 && timestamp(v.created_at) && timestamp(v.updated_at);
}
function isMessage(v: unknown): v is SupportMessage {
  return object(v) && exact(v, ["id", "actor_id", "body", "created_at"]) && typeof v.id === "string" && auditUUID.test(v.id) &&
    typeof v.actor_id === "string" && auditUUID.test(v.actor_id) && typeof v.body === "string" && v.body.length >= 1 && v.body.length <= 2000 && timestamp(v.created_at);
}
function envelope(v: unknown): v is { data: unknown; meta: { trace_id: string; next_cursor?: string | null } } {
  return object(v) && exact(v, ["data", "meta"]) && object(v.meta) && typeof v.meta.trace_id === "string";
}
export function parseSupportPage(v: unknown): { cases: SupportCase[]; next: string | null } | null {
  if (!envelope(v) || !exact(v.meta, ["trace_id", "next_cursor"]) || !Array.isArray(v.data) || v.data.length > 50 || !v.data.every(isSupportCase) ||
    v.meta.next_cursor !== null && (typeof v.meta.next_cursor !== "string" || !/^[A-Za-z0-9_-]{1,384}$/.test(v.meta.next_cursor)) || new Set(v.data.map((row: SupportCase) => row.id)).size !== v.data.length) return null;
  return { cases: v.data, next: v.meta.next_cursor ?? null };
}
export function parseSupportDetail(v: unknown, id: string): { case: SupportCase; messages: SupportMessage[] } | null {
  if (!envelope(v) || !exact(v.meta, ["trace_id"]) || !object(v.data) || !exact(v.data, ["case", "messages"]) || !isSupportCase(v.data.case) || v.data.case.id !== id ||
    !Array.isArray(v.data.messages) || v.data.messages.length > 100 || !v.data.messages.every(isMessage) || new Set(v.data.messages.map((row: SupportMessage) => row.id)).size !== v.data.messages.length) return null;
  return { case: v.data.case, messages: v.data.messages };
}
export function parseSupportReceipt(v: unknown): { case_id: string; version: number; replay: boolean } | null {
  if (!envelope(v) || !exact(v.meta, ["trace_id"]) || !object(v.data) || !exact(v.data, ["case_id", "version", "replay"]) || typeof v.data.case_id !== "string" || !auditUUID.test(v.data.case_id) ||
    !Number.isSafeInteger(v.data.version) || (v.data.version as number) < 1 || typeof v.data.replay !== "boolean") return null;
  return v.data as { case_id: string; version: number; replay: boolean };
}
