import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import test from "node:test";

const hooks = registerHooks({ resolve(specifier, context, next) {
  if (specifier === "server-only") return { url: "data:text/javascript,export {}", shortCircuit: true };
  if (specifier.startsWith("@/")) return { url: new URL(`../../src/${specifier.slice(2)}.ts`, import.meta.url).href, shortCircuit: true };
  return next(specifier, context);
} });
const { proxyAuth } = await import("../../src/lib/server/auth-proxy.ts");
hooks.deregister();

test("auth acknowledgements preserve 202 without requiring upstream JSON", async () => {
  const previous = { fetch: globalThis.fetch, api: process.env.BIDORA_API_ORIGIN, web: process.env.BIDORA_WEB_ORIGIN, mode: process.env.BIDORA_BFF_CLIENT_IP_MODE };
  process.env.BIDORA_API_ORIGIN = "http://127.0.0.1:18080";
  process.env.BIDORA_WEB_ORIGIN = "http://127.0.0.1:13000";
  process.env.BIDORA_BFF_CLIENT_IP_MODE = "trusted-header";
  try {
    for (const operation of ["register", "password-recovery", "verification-resend", "login-code-resend"]) {
      globalThis.fetch = async () => new Response("Accepted", { status: 202, headers: { "Content-Type": "text/plain; charset=utf-8", "X-Bidora-Challenge-Expires-At": "2037-01-01T12:00:00Z", "Set-Cookie": "__Host-bidora_login_code=synthetic; Path=/; Secure; HttpOnly; Expires=Thu, 01 Jan 2037 12:10:00 GMT" } });
      const response = await proxyAuth(new Request("http://127.0.0.1:13000/api/auth/" + operation, { method: "POST", headers: { Origin: process.env.BIDORA_WEB_ORIGIN, "Content-Type": "application/json", "X-Bidora-Client-IP": "192.0.2.1" }, body: "{}" }), operation);
      assert.equal(response.status, 202, operation);
      assert.equal(await response.text(), "");
      assert.equal(response.headers.get("cache-control"), "private, no-store");
      if (operation === "login-code-resend") assert.equal(response.headers.get("x-bidora-challenge-expires-at"), "2037-01-01T12:00:00.000Z");
    }
  } finally {
    globalThis.fetch = previous.fetch;
    for (const [key, value] of [["BIDORA_API_ORIGIN", previous.api], ["BIDORA_WEB_ORIGIN", previous.web], ["BIDORA_BFF_CLIENT_IP_MODE", previous.mode]]) {
      if (value === undefined) delete process.env[key]; else process.env[key] = value;
    }
  }
});
