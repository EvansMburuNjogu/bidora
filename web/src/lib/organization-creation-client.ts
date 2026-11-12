import type { OrganizationCreationRequest, OrganizationCreationResponse } from "@/contracts/auth.generated";

export async function submitOrganizationCreation(
  payload: OrganizationCreationRequest,
  csrfToken: string,
  idempotencyKey: string,
): Promise<Response> {
  return fetch("/api/tenderpreneur/organizations", {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrfToken,
      "Idempotency-Key": idempotencyKey,
    },
    body: JSON.stringify(payload),
  });
}

export type OrganizationCreationStatus = { operation_id: string; tenant_id: string; name: string; state: "REQUESTED" | "PLAN_SELECTED" | "PROVISIONING" | "SUBSCRIPTION_ACTIVATION" | "ACTIVE" | "FAILED" | "REVIEW_REQUIRED" | "CANCELLED"; job_state: "WAITING_PAYMENT" | "PENDING" | "RUNNING" | "RETRYABLE" | "REVIEW_REQUIRED" | "COMPLETED" | "CANCELLED"; commercial_path: "TRIAL" | "PAID"; created_at: string; updated_at: string };
export function parseOrganizationCreationStatus(value: unknown): { status: OrganizationCreationStatus | null } | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const root = value as Record<string, unknown>;
  if (Object.keys(root).sort().join(",") !== "data,meta" || !root.meta || typeof root.meta !== "object" || typeof (root.meta as Record<string, unknown>).trace_id !== "string") return null;
  if (root.data === null) return { status: null };
  if (!root.data || typeof root.data !== "object" || Array.isArray(root.data)) return null;
  const data = root.data as Record<string, unknown>;
  if (Object.keys(data).sort().join(",") !== "commercial_path,created_at,job_state,name,operation_id,state,tenant_id,updated_at" || !isUUID(data.operation_id) || !isUUID(data.tenant_id) || typeof data.name !== "string" || !data.name || data.name.length > 300 || !["REQUESTED", "PLAN_SELECTED", "PROVISIONING", "SUBSCRIPTION_ACTIVATION", "ACTIVE", "FAILED", "REVIEW_REQUIRED", "CANCELLED"].includes(String(data.state)) || !["WAITING_PAYMENT", "PENDING", "RUNNING", "RETRYABLE", "REVIEW_REQUIRED", "COMPLETED", "CANCELLED"].includes(String(data.job_state)) || !["TRIAL", "PAID"].includes(String(data.commercial_path)) || typeof data.created_at !== "string" || !Number.isFinite(Date.parse(data.created_at)) || typeof data.updated_at !== "string" || !Number.isFinite(Date.parse(data.updated_at))) return null;
  return { status: data as OrganizationCreationStatus };
}

function isUUID(value: unknown): value is string {
  return typeof value === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

export function parseOrganizationCreation(value: unknown): OrganizationCreationResponse | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (!record.data || typeof record.data !== "object" || Array.isArray(record.data)) return null;
  const data = record.data as Record<string, unknown>;
  if (!isUUID(data.operation_id) || !isUUID(data.tenant_id) || data.state !== "REQUESTED") return null;
  const meta = record.meta;
  if (!meta || typeof meta !== "object" || Array.isArray(meta) || typeof (meta as Record<string, unknown>).trace_id !== "string") return null;
  return value as OrganizationCreationResponse;
}

export async function safeOrganizationCreationCode(response: Response): Promise<string> {
  try {
    const value = await response.json() as unknown;
    if (!value || typeof value !== "object" || Array.isArray(value)) return "ORGANIZATION_CREATION_UNAVAILABLE";
    const error = (value as Record<string, unknown>).error;
    if (!error || typeof error !== "object" || Array.isArray(error)) return "ORGANIZATION_CREATION_UNAVAILABLE";
    const code = (error as Record<string, unknown>).code;
    return typeof code === "string" ? code : "ORGANIZATION_CREATION_UNAVAILABLE";
  } catch {
    return "ORGANIZATION_CREATION_UNAVAILABLE";
  }
}
