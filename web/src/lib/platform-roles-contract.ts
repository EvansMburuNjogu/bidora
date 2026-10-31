export const fixedPlatformRoles = ["PLATFORM_OWNER", "OPERATIONS_ADMINISTRATOR", "BILLING_ADMINISTRATOR", "INTEGRATION_ADMINISTRATOR", "SUPPORT_ADMINISTRATOR", "SECURITY_AUDIT_ADMINISTRATOR"] as const;
export type FixedPlatformRole = (typeof fixedPlatformRoles)[number];
export type PlatformRoleCatalogue = { roles: { role_key: FixedPlatformRole; catalogue_version: 1; capabilities: string[] }[]; custom_roles_supported: false; tenant_access_implied: false };
function record(v: unknown): Record<string, unknown> | null { return v !== null && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : null; }
function exact(v: Record<string, unknown>, fields: readonly string[]): boolean { return Object.keys(v).length === fields.length && fields.every((field) => Object.hasOwn(v, field)); }
export function parsePlatformRoleCatalogue(value: unknown): PlatformRoleCatalogue | null {
  const root = record(value), data = record(root?.data), meta = record(root?.meta);
  if (!root || !data || !meta || !exact(root, ["data", "meta"]) || !(exact(meta, ["trace_id"]) || exact(meta, ["trace_id", "next_cursor"]) && meta.next_cursor === null) ||
      typeof meta.trace_id !== "string" || !meta.trace_id || !exact(data, ["roles", "custom_roles_supported", "tenant_access_implied"]) ||
      data.custom_roles_supported !== false || data.tenant_access_implied !== false || !Array.isArray(data.roles) || data.roles.length !== 6) return null;
  const seen = new Set<string>(), roles: PlatformRoleCatalogue["roles"] = [];
  for (const value of data.roles) {
    const row = record(value);
    if (!row || !exact(row, ["role_key", "catalogue_version", "capabilities"]) || !fixedPlatformRoles.some((role) => role === row.role_key) ||
        seen.has(row.role_key as string) || row.catalogue_version !== 1 || !Array.isArray(row.capabilities) || row.capabilities.length < 1 || row.capabilities.length > 100 ||
        row.capabilities.some((cap) => typeof cap !== "string" || cap.length > 150 || !/^platform\.[a-z._]+$/.test(cap)) || new Set(row.capabilities).size !== row.capabilities.length) return null;
    seen.add(row.role_key as string);
    roles.push({ role_key: row.role_key as FixedPlatformRole, catalogue_version: 1, capabilities: row.capabilities as string[] });
  }
  return { roles, custom_roles_supported: false, tenant_access_implied: false };
}
