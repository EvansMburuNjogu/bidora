import type {
  MFAActivationResponse,
  MFAEnrollmentResponse,
  MFAReadinessResponse,
  MFARecoveryCodesResponse,
} from "@/contracts/auth.generated";

function object(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function exact(value: Record<string, unknown>, keys: string[]): boolean {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}

function uuid(value: unknown): value is string {
  return typeof value === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

function instant(value: unknown): value is string {
  return typeof value === "string" && value.length <= 40 && /^\d{4}-\d{2}-\d{2}T/.test(value) && Number.isFinite(Date.parse(value));
}

function meta(value: unknown): value is { trace_id: string } {
  const record = object(value);
  return !!record && exact(record, ["trace_id"]) && uuid(record.trace_id);
}

export function parseMFAReadiness(value: unknown): MFAReadinessResponse | null {
  const root = object(value);
  const data = object(root?.data);
  const keys = [
    "email_verified", "account_disabled", "password_credential_present", "google_identity_present",
    "mfa_enrollment_available", "mfa_enrolled", "step_up_available", "step_up_policy_configured",
    "session_assurance_provenance_available", "recovery_codes_available", "session_authenticated_at",
    "session_age_seconds", "blockers", "observed_at",
  ];
  if (!root || !exact(root, ["data", "meta"]) || !data || !exact(data, keys) || !meta(root.meta)) return null;
  const booleans = keys.slice(0, 10);
  if (booleans.some((key) => typeof data[key] !== "boolean")) return null;
  if (!instant(data.session_authenticated_at) || !instant(data.observed_at)) return null;
  if (!Number.isSafeInteger(data.session_age_seconds) || Number(data.session_age_seconds) < 0) return null;
  if (!Array.isArray(data.blockers) || data.blockers.length > 20 || data.blockers.some((item) => typeof item !== "string" || item.length > 100)) return null;
  return value as MFAReadinessResponse;
}

export function parseMFAEnrollment(value: unknown): MFAEnrollmentResponse | null {
  const root = object(value);
  const data = object(root?.data);
  if (!root || !exact(root, ["data", "meta"]) || !data || !exact(data, ["factor_id", "secret", "otpauth_uri", "expires_at"]) || !meta(root.meta)) return null;
  if (!uuid(data.factor_id) || typeof data.secret !== "string" || !/^[A-Z2-7]{32}$/.test(data.secret)) return null;
  if (typeof data.otpauth_uri !== "string" || data.otpauth_uri.length > 2048 || !data.otpauth_uri.startsWith("otpauth://totp/")) return null;
  if (!instant(data.expires_at)) return null;
  return value as MFAEnrollmentResponse;
}

function validRecoveryCodes(value: unknown): value is string[] {
  return Array.isArray(value) && value.length === 10 && new Set(value).size === 10 && value.every((code) => typeof code === "string" && /^[0-9A-HJKMNP-TV-Z]{16}$/.test(code));
}

export function parseMFARecoveryCodes(value: unknown): MFARecoveryCodesResponse | null {
  const root = object(value);
  const data = object(root?.data);
  if (!root || !exact(root, ["data", "meta"]) || !data || !exact(data, ["recovery_codes"]) || !meta(root.meta) || !validRecoveryCodes(data.recovery_codes)) return null;
  return value as MFARecoveryCodesResponse;
}

export function parseMFAActivation(value: unknown): MFAActivationResponse | null {
  const root = object(value);
  const data = object(root?.data);
  if (!root || !exact(root, ["data", "meta"]) || !data || !exact(data, ["recovery_codes", "assurance_level", "assurance_method"]) || !meta(root.meta)) return null;
  if (data.assurance_level !== "AAL2" || data.assurance_method !== "TOTP") return null;
  if (!validRecoveryCodes(data.recovery_codes)) return null;
  return value as MFAActivationResponse;
}
