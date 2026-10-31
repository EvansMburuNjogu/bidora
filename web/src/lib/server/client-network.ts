import "server-only";
import { isIP } from "node:net";

export type ClientNetworkResult = { ok: true; address: string | null } | { ok: false };

// Development uses the direct Next.js peer and forwards no caller-controlled
// network header. Production requires an ingress that prevents direct access to
// Next.js and strips/overwrites X-Bidora-Client-IP before the request arrives.
export function clientNetwork(request: Request): ClientNetworkResult {
  const mode = process.env.BIDORA_BFF_CLIENT_IP_MODE;
  if (process.env.NODE_ENV !== "production" && (!mode || mode === "direct-peer")) {
    return { ok: true, address: null };
  }
  if (mode !== "trusted-header") return { ok: false };
  const value = request.headers.get("x-bidora-client-ip");
  if (!value || value !== value.trim() || value.includes(",") || isIP(value) === 0) return { ok: false };
  return { ok: true, address: value };
}
