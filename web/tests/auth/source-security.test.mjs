import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const root = path.resolve(import.meta.dirname, "../..");

async function filesUnder(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map((entry) => entry.isDirectory() ? filesUnder(path.join(directory, entry.name)) : [path.join(directory, entry.name)]));
  return nested.flat();
}

test("authentication and session source never persists security state in browser storage", async () => {
  const files = (await filesUnder(path.join(root, "src"))).filter((file) => /\.(ts|tsx)$/.test(file));
  const combined = (await Promise.all(files.map((file) => readFile(file, "utf8")))).join("\n");
  assert.doesNotMatch(combined, /\blocalStorage\b|\bsessionStorage\b/);
});

test("password confirmations remain client-only", async () => {
  const form = await readFile(path.join(root, "src/components/auth/auth-form.tsx"), "utf8");
  const change = await readFile(path.join(root, "src/components/auth/password-change-form.tsx"), "utf8");
  const client = await readFile(path.join(root, "src/lib/auth-client.ts"), "utf8");
  assert.match(form, /password !== confirmPassword/);
  assert.match(change, /newPassword !== confirmPassword/);
  assert.doesNotMatch(client, /confirm_password/);
});

test("TOTP verification uses six accessible digits without changing the backend contract", async () => {
  const input = await readFile(path.join(root, "src/components/auth/verification-code-input.tsx"), "utf8");
  assert.match(input, /const codeLength = 6/);
  assert.match(input, /name="code"/);
  assert.match(input, /Digit \$\{index \+ 1\} of \$\{codeLength\}/);
  assert.match(input, /onPaste=\{paste\}/);
});

test("account security exposes the existing password-confirmed Google linking workflow", async () => {
  const account = await readFile(path.join(root, "src/components/auth/account-security.tsx"), "utf8");
  const linking = await readFile(path.join(root, "src/components/auth/google-linking.tsx"), "utf8");
  assert.match(account, /<GoogleLinking\s*\/>/);
  assert.match(linking, /\/api\/auth\/google-link\/start/);
  assert.match(linking, /password/);
});

test("Google and BFF boundaries retain fixed destinations and fail-closed client-network mode", async () => {
  const google = await readFile(path.join(root, "src/lib/server/google-link-proxy.ts"), "utf8");
  const auth = await readFile(path.join(root, "src/lib/server/auth-proxy.ts"), "utf8");
  const network = await readFile(path.join(root, "src/lib/server/client-network.ts"), "utf8");
  assert.match(google, /https:\/\/accounts\.google\.com/);
  assert.match(auth, /https:\/\/accounts\.google\.com/);
  assert.match(network, /mode !== "trusted-header"/);
  assert.match(network, /isIP\(value\) === 0/);
});
