export type PlatformSessionPolicy = { idle_seconds: number; absolute_seconds: number; version: number };
export function parsePlatformSessionPolicy(value: unknown): PlatformSessionPolicy | null {
  if (!value || typeof value !== "object" || Object.keys(value).sort().join(",") !== "data,meta" || !("data" in value) || !("meta" in value)) return null;
  const { data, meta } = value;
  if (!data || typeof data !== "object" || Object.keys(data).sort().join(",") !== "absolute_seconds,idle_seconds,version" ||
      !meta || typeof meta !== "object" || !("trace_id" in meta) || typeof meta.trace_id !== "string") return null;
  const p = data as PlatformSessionPolicy;
  return [300, 600, 900, 1800].includes(p.idle_seconds) && [3600, 14400, 28800, 43200].includes(p.absolute_seconds) &&
    p.absolute_seconds >= p.idle_seconds && Number.isSafeInteger(p.version) && p.version > 0 ? p : null;
}
