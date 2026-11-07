export type SecurityControls = {
  failed_login_attempts: number;
  failure_window_seconds: number;
  lockout_seconds: number;
  minimum_password_bytes: number;
  password_reset_seconds: number;
  allow_multiple_sessions: boolean;
  admin_idle_seconds: number;
  admin_absolute_seconds: number;
};
export type PlatformSessionPolicy = { idle_seconds: number; absolute_seconds: number; security: SecurityControls; version: number };
const securityKeys = ["failed_login_attempts", "failure_window_seconds", "lockout_seconds", "minimum_password_bytes", "password_reset_seconds", "allow_multiple_sessions", "admin_idle_seconds", "admin_absolute_seconds"].sort().join(",");
const bounded = (n: unknown, min: number, max: number): n is number => typeof n === "number" && Number.isSafeInteger(n) && n >= min && n <= max;
export function validSecurityControls(value: unknown): value is SecurityControls {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).sort().join(",") !== securityKeys) return false;
  const p = value as SecurityControls;
  return bounded(p.failed_login_attempts, 1, 5) && bounded(p.failure_window_seconds, 60, 3600) && bounded(p.lockout_seconds, 60, 3600) && bounded(p.minimum_password_bytes, 12, 128) && bounded(p.password_reset_seconds, 300, 1800) && typeof p.allow_multiple_sessions === "boolean" && bounded(p.admin_idle_seconds, 300, 604800) && bounded(p.admin_absolute_seconds, 3600, 604800) && p.admin_absolute_seconds >= p.admin_idle_seconds && [p.failure_window_seconds, p.lockout_seconds, p.password_reset_seconds, p.admin_idle_seconds, p.admin_absolute_seconds].every(n => n % 60 === 0);
}
export function validSessionPolicy(p: PlatformSessionPolicy): boolean {
  return [300, 600, 900, 1800].includes(p.idle_seconds) && [3600, 14400, 28800, 43200].includes(p.absolute_seconds) && p.absolute_seconds >= p.idle_seconds && bounded(p.version, 1, Number.MAX_SAFE_INTEGER) && validSecurityControls(p.security);
}
export function parsePlatformSessionPolicy(value: unknown): PlatformSessionPolicy | null {
  if (!value || typeof value !== "object" || Object.keys(value).sort().join(",") !== "data,meta" || !("data" in value) || !("meta" in value)) return null;
  const { data, meta } = value;
  if (!data || typeof data !== "object" || Object.keys(data).sort().join(",") !== "absolute_seconds,idle_seconds,security,version" || !meta || typeof meta !== "object" || !("trace_id" in meta) || typeof meta.trace_id !== "string") return null;
  return validSessionPolicy(data as PlatformSessionPolicy) ? data as PlatformSessionPolicy : null;
}
