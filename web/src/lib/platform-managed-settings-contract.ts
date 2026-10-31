export type GoogleSettings = { id: string; version: number; enabled: boolean; allow_signup: boolean; client_id: string; allowed_domains: string[]; secret_configured: boolean; connection_status: string; runtime_configured: boolean; created_at: string };
export type NotificationTemplate = { template_key: string; name: string; category: string; channel: "EMAIL"; mandatory: boolean; enabled: boolean; subject: string; body: string; version: number; updated_at: string; placeholders: string[]; test_available: boolean };
const object = (v: unknown): Record<string, unknown> | null => v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null;
const exact = (v: object, keys: string[]) => Object.keys(v).sort().join(",") === [...keys].sort().join(",");
export function settingsData(value: unknown): unknown { const root = object(value), meta = object(root?.meta); return root && exact(root, ["data", "meta"]) && meta && typeof meta.trace_id === "string" ? root.data : null; }
export function parseGoogleSettings(value: unknown): GoogleSettings | null {
  const v = object(value); if (!v || !exact(v, ["id", "version", "enabled", "allow_signup", "client_id", "allowed_domains", "secret_configured", "connection_status", "runtime_configured", "created_at"]) || typeof v.id !== "string" || !/^[0-9a-f-]{36}$/.test(v.id) || !Number.isSafeInteger(v.version) || Number(v.version) < 1 || typeof v.enabled !== "boolean" || typeof v.allow_signup !== "boolean" || typeof v.client_id !== "string" || v.client_id.length > 255 || !Array.isArray(v.allowed_domains) || v.allowed_domains.length > 50 || v.allowed_domains.some((d) => typeof d !== "string" || d.length > 253 || !/^[a-z0-9.-]+$/.test(d)) || typeof v.secret_configured !== "boolean" || typeof v.runtime_configured !== "boolean" || !["UNTESTED", "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED", "UNAVAILABLE"].includes(String(v.connection_status)) || typeof v.created_at !== "string" || !Number.isFinite(Date.parse(v.created_at))) return null;
  return v as GoogleSettings;
}
export function parseNotificationTemplate(value: unknown): NotificationTemplate | null {
  const v = object(value); if (!v || !exact(v, ["template_key", "name", "category", "channel", "mandatory", "enabled", "subject", "body", "version", "updated_at", "placeholders", "test_available"]) || typeof v.template_key !== "string" || !/^[a-z][a-z0-9_.-]{0,99}$/.test(v.template_key) || typeof v.name !== "string" || typeof v.category !== "string" || v.channel !== "EMAIL" || typeof v.mandatory !== "boolean" || typeof v.enabled !== "boolean" || v.mandatory && !v.enabled || typeof v.subject !== "string" || typeof v.body !== "string" || !Number.isSafeInteger(v.version) || Number(v.version) < 1 || typeof v.updated_at !== "string" || !Number.isFinite(Date.parse(v.updated_at)) || !Array.isArray(v.placeholders) || v.placeholders.some((p) => typeof p !== "string" || !/^[a-z_]+$/.test(p)) || typeof v.test_available !== "boolean") return null;
  return v as NotificationTemplate;
}
export function googleDraftErrors(draft: { enabled: boolean; client_id: string; client_secret: string; allowed_domains: string[] }, secretRequired: boolean): Partial<Record<"client_id" | "client_secret" | "allowed_domains", string>> {
  const errors: Partial<Record<"client_id" | "client_secret" | "allowed_domains", string>> = {};
  if (draft.client_id.length > 255 || draft.enabled && !draft.client_id || draft.client_id && !/^[0-9]+-[a-z0-9_-]+\.apps\.googleusercontent\.com$/.test(draft.client_id)) errors.client_id = "Enter a valid Google Client ID.";
  const size = new TextEncoder().encode(draft.client_secret).length;
  if (secretRequired && !size || size && (size < 8 || size > 1024 || draft.client_secret.trim() !== draft.client_secret || /[\r\n\u0000]/.test(draft.client_secret))) errors.client_secret = "Enter a replacement secret between 8 and 1,024 bytes.";
  if (draft.allowed_domains.length > 50 || new Set(draft.allowed_domains).size !== draft.allowed_domains.length || draft.allowed_domains.some((d) => d.length > 253 || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$/.test(d) || d.split(".").some((p) => p.length > 63))) errors.allowed_domains = "Enter up to 50 unique exact email domains.";
  return errors;
}
export function notificationDraftErrors(template: Pick<NotificationTemplate, "template_key" | "mandatory" | "placeholders">, draft: { name: string; subject: string; body: string; enabled: boolean }): Partial<Record<"name" | "subject" | "body" | "enabled", string>> {
  const errors: Partial<Record<"name" | "subject" | "body" | "enabled", string>> = {};
  const bytes = (v: string) => new TextEncoder().encode(v).length;
  if (!draft.name.trim() || draft.name.trim() !== draft.name || bytes(draft.name) > 80) errors.name = "Enter a name of at most 80 bytes without surrounding spaces.";
  if (!draft.subject.trim() || draft.subject.trim() !== draft.subject || bytes(draft.subject) > 200 || /[\u0000-\u001f\u007f]/.test(draft.subject) || /\{\{(?:token|code)\}\}/.test(draft.subject)) errors.subject = "Enter a subject of at most 200 bytes without codes or tokens.";
  if (!draft.body.trim() || bytes(draft.body) > 4000) errors.body = "Enter a message of at most 4,000 bytes.";
  for (const field of ["subject", "body"] as const) {
    const remaining = draft[field].replace(/\{\{([a-z_]+)\}\}/g, (_, key: string) => { if (!template.placeholders.includes(key)) errors[field] = "Use only the supported placeholders."; return ""; });
    if (remaining.includes("{{") || remaining.includes("}}")) errors[field] = "Use placeholders exactly as shown.";
  }
  const required: Record<string, string[]> = { "identity.email_verification": ["token", "expires_at"], "identity.password_recovery": ["token", "expires_at"], "identity.login_code": ["code", "expires_at"], "tenant.invitation": ["token", "expires_at", "role"], "platform.invitation.issued": ["token", "expires_at", "role"], "platform.invitation.reminder": ["role", "expires_at"], "platform.session_revocation": ["occurred_at"], "platform.identity_state": ["action", "occurred_at"], "platform.role_change": ["role", "action", "occurred_at"] };
  if (!required[template.template_key] || required[template.template_key].some((p) => !draft.body.includes(`{{${p}}}`))) errors.body = "Keep all required message placeholders.";
  if (template.mandatory && !draft.enabled) errors.enabled = "This notification is required.";
  return errors;
}
export function parseTemplateList(value: unknown): NotificationTemplate[] | null { const data = settingsData(value); if (!Array.isArray(data) || data.length > 100) return null; const rows = data.map(parseNotificationTemplate); return rows.some((row) => row === null) ? null : rows as NotificationTemplate[]; }
