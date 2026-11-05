import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { registerHooks } from "node:module";
import test from "node:test";

const hooks = registerHooks({
  resolve(specifier, context, next) {
    return specifier.startsWith("@/")
      ? next(new URL(`../src/${specifier.slice(2)}.ts`, import.meta.url).href, context)
      : next(specifier, context);
  },
});
const { permissionMatrix, fallbackRoles, granted } = await import("../src/lib/platform-role-matrix.ts");
const { fixedRoleCapabilities } = await import("../src/lib/platform-fixed-role-bundles.generated.ts");
hooks.deregister();

const rows = permissionMatrix.flatMap((group) => group.rows);
test("matrix covers every backend capability exactly once without invented permissions", async () => {
  const backend = await readFile(new URL("../../backend/internal/platformadmin/domain/role.go", import.meta.url), "utf8");
  const capabilities = [...new Set([...backend.matchAll(/"(platform\.[a-z._]+)"/g)].map((match) => match[1]))].sort();
  assert.ok(capabilities.length > 50, "read the actual backend capability catalogue");
  assert.equal(new Set(permissionMatrix.map((group) => group.name)).size, permissionMatrix.length);
  for (const group of permissionMatrix) {
    assert.ok(group.name.trim() && group.rows.length);
    for (const row of group.rows) {
      assert.ok(row.label.trim());
      assert.equal(row.keys.length, 1, `${row.label} must identify one real permission`);
    }
  }
  const represented = rows.flatMap((row) => row.keys);
  assert.equal(new Set(represented).size, represented.length, "permissions must not be duplicated or combined");
  assert.deepEqual([...represented].sort(), capabilities);
  assert.deepEqual([...new Set(Object.values(fixedRoleCapabilities).flat())].sort(), capabilities);
});
test("fallback grants stay identical to the published backend role bundles", async () => {
  const contract = JSON.parse(await readFile(new URL("../../backend/api/openapi/drafts/platform-administrator-lifecycle.json", import.meta.url), "utf8"));
  const bundles = contract["x-role-capability-bundles"];
  assert.deepEqual(Object.keys(fixedRoleCapabilities).sort(), Object.keys(bundles).sort());
  for (const role of fallbackRoles) {
    assert.deepEqual([...role.capabilities].sort(), [...bundles[role.role_key]].sort());
    for (const row of rows) {
      assert.equal(granted(role.capabilities, row.keys), bundles[role.role_key].includes(row.keys[0]), `${role.role_key}: ${row.label}`);
    }
  }
});
test("display uses supplied live capabilities, never hardcoded Owner privileges", () => {
  const singleGrant = ["platform.integration.smtp.test"];
  assert.equal(rows.filter((row) => granted(singleGrant, row.keys)).length, 1);
  assert.equal(granted([], ["platform.admin.role.assign"]), false);
  assert.equal(granted(singleGrant, []), false);
  assert.equal(granted(singleGrant, ["platform.integration.smtp.activate"]), false);
  assert.equal(granted(singleGrant, ["platform.integration.smtp.test", "platform.integration.smtp.activate"]), false);
});
