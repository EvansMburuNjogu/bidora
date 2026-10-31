import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { hasPlatformNavigationAccess, platformNavigation } from "../src/components/platform/platform-navigation.ts";

test("official Platform menu has eleven ordered routes; preview-only modules confer no data authority", () => {
  assert.deepEqual(platformNavigation.map((group) => group.label), ["Workspace", "Directory", "Operations", "Insights", "Administration"]);
  assert.ok(platformNavigation.every((group) => group.items.length > 0));
  const items = platformNavigation.flatMap((group) => group.items);
  assert.deepEqual(items.map((item) => item.label), ["Overview", "Tenderpreneurs", "Users", "Plans & Subscriptions", "Billing", "AI Usage", "Communications", "Support", "Reports", "Audit Trails", "Settings"]);
  const paths = new Set();
  for (const item of items) {
    assert.equal(paths.has(item.href), false, `duplicate ${item.href}`);
    paths.add(item.href);
    assert.equal(existsSync(join(import.meta.dirname, "../src/app/(session)", item.href, "page.tsx")), true, `missing ${item.href}`);
    assert.equal(hasPlatformNavigationAccess(item, new Set()), item.available && item.capabilities.length === 0);
    assert.equal(hasPlatformNavigationAccess(item, new Set(item.capabilities)), item.available);
  }
  assert.equal(paths.has("/platform/settings/smtp"), false);
  assert.equal(paths.has("/platform/settings"), true);
});
