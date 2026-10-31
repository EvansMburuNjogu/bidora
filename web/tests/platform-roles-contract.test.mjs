import assert from "node:assert/strict";
import test from "node:test";
import { fixedPlatformRoles, parsePlatformRoleCatalogue } from "../src/lib/platform-roles-contract.ts";
const roles = fixedPlatformRoles.map((role_key) => ({ role_key, catalogue_version: 1, capabilities: ["platform.admin.read"] }));
const meta = { trace_id: "safe-trace" };
test("fixed roles require exact catalogue version and cannot introduce editable or tenant-authority claims", () => {
  const data = { roles, custom_roles_supported: false, tenant_access_implied: false };
  assert.equal(parsePlatformRoleCatalogue({ data, meta })?.roles.length, 6);
  assert.equal(parsePlatformRoleCatalogue({ data: { ...data, custom_roles_supported: true }, meta }), null);
  assert.equal(parsePlatformRoleCatalogue({ data: { ...data, tenant_access_implied: true }, meta }), null);
  assert.equal(parsePlatformRoleCatalogue({ data: { ...data, roles: [roles[0], ...roles.slice(0, 5)] }, meta }), null);
  assert.equal(parsePlatformRoleCatalogue({ data: { ...data, roles: [{ ...roles[0], catalogue_version: 2 }, ...roles.slice(1)] }, meta }), null);
  assert.equal(parsePlatformRoleCatalogue({ data: { ...data, roles: [{ ...roles[0], secret: "private" }, ...roles.slice(1)] }, meta }), null);
});
