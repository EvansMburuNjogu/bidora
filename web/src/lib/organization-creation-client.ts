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
