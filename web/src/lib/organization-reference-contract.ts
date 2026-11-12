import { settingsData } from "@/lib/platform-managed-settings-contract";

export type OrganizationReference = { kind: "BUSINESS_TYPE" | "INDUSTRY"; code: string; label: string; enabled: boolean; version: number; updated_at: string };
export type OrganizationReferenceKind = "business-types" | "industries";
export function parseOrganizationReferences(value: unknown, kind: OrganizationReferenceKind, activeOnly = false): OrganizationReference[] | null {
  const data = settingsData(value);
  if (!Array.isArray(data) || data.length > 500) return null;
  const expected = kind === "business-types" ? "BUSINESS_TYPE" : "INDUSTRY";
  const codes = new Set<string>();
  for (const value of data) {
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const row = value as Record<string, unknown>;
    if (Object.keys(row).sort().join(",") !== "code,enabled,kind,label,updated_at,version" || row.kind !== expected || typeof row.code !== "string" || row.code.length > 64 || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(row.code) || codes.has(row.code) || typeof row.label !== "string" || !row.label.trim() || row.label.trim() !== row.label || new TextEncoder().encode(row.label).length > 100 || /[\u0000-\u001f\u007f]/.test(row.label) || typeof row.enabled !== "boolean" || activeOnly && !row.enabled || !Number.isSafeInteger(row.version) || Number(row.version) < 1 || typeof row.updated_at !== "string" || !Number.isFinite(Date.parse(row.updated_at))) return null;
    codes.add(row.code);
  }
  return data as OrganizationReference[];
}
