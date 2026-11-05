import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import test from "node:test";
import { parseAuditRecord, auditChangeSummary } from "../src/lib/platform-audit-contract.ts";
const record = { id: "11111111-1111-4111-8111-111111111111", administrator_id: "22222222-2222-4222-8222-222222222222", action: "PlatformInformationUpdated", module: "ADMINISTRATION", affected_record_id: null, result: "SUCCESSFUL", occurred_at: "2026-01-01T12:00:00Z" };
const delta = { field: "contact_phone", operation: "DELETED", before: "+254 700 000 001", after: null };
test("audit details accept safe labels, restricted metadata and explicit changes", () => {
  const enriched = { ...record, administrator_name: "Example Admin", affected_record_name: "Platform information", ip_address: "2001:db8::1", browser: "Firefox", changes: [delta] };
  assert.deepEqual(parseAuditRecord(enriched), enriched);
  assert.deepEqual(parseAuditRecord(record), record, "old records remain readable without invented metadata");
  assert.equal(auditChangeSummary(enriched), "Deleted contact phone");
  assert.equal(auditChangeSummary(record), "No field details recorded");
  for (const changes of [[{ ...delta, field: "password" }], [{ ...delta, operation: "EXECUTED" }], [{ ...delta, field: "client_secret", after: "secret", before: null }], [{ ...delta, after: "x".repeat(513) }], [{ ...delta, after: "bad\nvalue" }]]) assert.equal(parseAuditRecord({ ...enriched, changes }), null);
  assert.equal(parseAuditRecord({ ...enriched, changes: [{ field: "client_secret", operation: "MODIFIED", before: null, after: "[redacted]" }] })?.changes.length, 1);
  for (const change of [{ browser: "x".repeat(41) }, { ip_address: "not an IP" }, { administrator_name: "bad\nname" }, { raw_summary: "private" }]) assert.equal(parseAuditRecord({ ...enriched, ...change }), null);
});
test("audit context forwards only ingress-validated IP and bounded user agent", async () => {
  const hooks = registerHooks({ resolve(specifier, context, next) {
    if (specifier === "server-only") return { url: "data:text/javascript,export {}", shortCircuit: true };
    return specifier.startsWith("@/") ? next(new URL(`../src/${specifier.slice(2)}.ts`, import.meta.url).href, context) : next(specifier, context);
  } });
  const previous = process.env.BIDORA_BFF_CLIENT_IP_MODE;
  try {
    const { forwardAuditContext } = await import("../src/lib/server/audit-request-context.ts");
    process.env.BIDORA_BFF_CLIENT_IP_MODE = "trusted-header";
    const request = new Request("https://example.test", { headers: { "x-bidora-client-ip": "192.0.2.20", "x-forwarded-for": "198.51.100.99", "user-agent": "Mozilla/5.0 Firefox/123" } });
    const headers = new Headers(); forwardAuditContext(request, headers);
    assert.equal(headers.get("x-forwarded-for"), "192.0.2.20");
    assert.equal(headers.get("user-agent"), "Mozilla/5.0 Firefox/123");
    const spoofed = new Headers(); forwardAuditContext(new Request("https://example.test", { headers: { "x-forwarded-for": "198.51.100.99", "user-agent": "x".repeat(513) } }), spoofed);
    assert.equal(spoofed.get("x-forwarded-for"), null); assert.equal(spoofed.get("user-agent"), null);
  } finally { hooks.deregister(); if (previous === undefined) delete process.env.BIDORA_BFF_CLIENT_IP_MODE; else process.env.BIDORA_BFF_CLIENT_IP_MODE = previous; }
});
